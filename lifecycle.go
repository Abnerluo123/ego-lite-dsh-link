package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The MCP server lives across model turns. It cannot infer task completion from
// inactivity, stdio EOF, a tool error, or the model's final response. Completion
// must be explicit; its receipt verifies the addressed space, not every agent.
func completionSource(ref, keep string) string {
	return `const spaces=await listTaskSpaces();
 const target=spaces.find(s=>s.id===` + ref + `);
 if(!target)return {spaceId:` + ref + `,status:'already-released',agentControlReleased:true,verification:'space-absent',kept:[],remainingAgentSpaces:spaces.filter(s=>s.ownership==='agent').map(s=>({id:s.id,name:s.name}))};
 if(target.ownership!=='agent')return {spaceId:target.id,status:'already-released',ownership:target.ownership,agentControlReleased:target.ownership==='user',verification:'ownership',kept:[],remainingAgentSpaces:spaces.filter(s=>s.ownership==='agent').map(s=>({id:s.id,name:s.name}))};
 const task=await taskSpace(target.id);
 const tabs=await task.tabs();
 const managed=tabs.filter(t=>t.label);
 const requested=` + keep + `;
 const keep=requested==='all'?managed.map(t=>t.label):[...new Set([...requested,...managed.filter(t=>t.openedBy==='unknown').map(t=>t.label)])];
 const labels=new Set(managed.map(t=>t.label));
 for(const label of keep)if(!labels.has(label))throw new Error('Cannot keep missing page '+label+'; inspect tabs first');
 await task.finish({keep:keep});
 // Page labels are invalid after finish. Only inspect the space registry here.
 let after;
 try{after=await listTaskSpaces()}catch(error){return {spaceId:target.id,status:'finished',kept:keep,agentControlReleased:null,verification:'unavailable',verificationError:String(error.message).split('\n')[0],nextAction:'Call ego_lite_control_status(spaceId) before claiming control was released'};}
 const retained=after.find(s=>s.id===target.id);
 const released=!retained||retained.ownership==='user';
 return {spaceId:target.id,status:released?'finished':'release-unverified',kept:keep,ownership:retained?.ownership||'closed',agentControlReleased:released,verification:'space-registry',remainingAgentSpaces:after.filter(s=>s.ownership==='agent').map(s=>({id:s.id,name:s.name})),...(released?{}:{nextAction:'Control still appears active. Do not report release success; inspect control_status.'})};`
}

func parseKeep(raw string) (string, error) {
	if strings.TrimSpace(raw) == "all" {
		return quote("all"), nil
	}
	labels := []string{}
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' }) {
		label, err := validatePage(part)
		if err != nil {
			return "", err
		}
		if !seen[label] {
			labels = append(labels, label)
			seen[label] = true
		}
	}
	data, _ := json.Marshal(labels)
	return string(data), nil
}
// spaceSummary renders the browser registry so a stale or guessed identifier
// can be rejected with the identifiers that actually exist.
func spaceSummary(entries []map[string]any, current int, currentName string) string {
	var agentIDs []string
	for _, entry := range entries {
		if entry["ownership"] == "agent" {
			agentIDs = append(agentIDs, fmt.Sprintf("%v (%v)", entry["id"], entry["name"]))
		}
	}
	parts := []string{}
	if current > 0 {
		if currentName != "" {
			parts = append(parts, fmt.Sprintf("current space is %d (%s)", current, currentName))
		} else {
			parts = append(parts, fmt.Sprintf("current space is %d", current))
		}
	}
	if len(agentIDs) > 0 {
		parts = append(parts, "agent spaces: "+strings.Join(agentIDs, ", "))
	} else {
		parts = append(parts, "no agent spaces exist")
	}
	return strings.Join(parts, "; ")
}

func (s *server) knownSpaces() ([]map[string]any, error) {
	value, err := runStandalone("const list=await listTaskSpaces();return list.map(s=>({id:s.id,name:s.name,ownership:s.ownership}));", 30*time.Second)
	if err != nil {
		return nil, err
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, errors.New("invalid space registry response")
	}
	entries := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if entry, ok := item.(map[string]any); ok {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// resolveSpace accepts an id, an exact space name, or nothing for the current
// space. Unknown ids are rejected with the ids that actually exist; when the
// registry cannot be read the id is used as given, because the caller's own
// receipt still validates the result.
func (s *server) resolveSpace(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return s.spaceID, nil
	}
	numeric, convErr := strconv.Atoi(value)
	if convErr == nil && numeric < 1 {
		return 0, fmt.Errorf("spaceId must be 1 or greater, got %d", numeric)
	}
	entries, listErr := s.knownSpaces()
	if listErr != nil {
		if convErr == nil {
			return numeric, nil
		}
		return 0, listErr
	}
	if convErr == nil {
		for _, entry := range entries {
			if id, ok := entry["id"].(float64); ok && int(id) == numeric {
				return numeric, nil
			}
		}
		return 0, fmt.Errorf("space %d does not exist (ids are never reused); %s; omit spaceId to address the current space, or call ego_lite_spaces", numeric, spaceSummary(entries, s.spaceID, s.taskName))
	}
	for _, entry := range entries {
		if name, _ := entry["name"].(string); name == value {
			if id, ok := entry["id"].(float64); ok {
				return int(id), nil
			}
		}
	}
	return 0, fmt.Errorf("no space is named %q; %s", value, spaceSummary(entries, s.spaceID, s.taskName))
}

func (s *server) completeSpace(rawID, keep string) (any, error) {
	id := s.spaceID
	explicit := strings.TrimSpace(rawID) != ""
	if explicit {
		var err error
		id, err = s.resolveSpace(rawID)
		if err != nil {
			return nil, err
		}
	}
	if id == 0 {
		if s.taskName != "" {
			return nil, errors.New("active space has no numeric id; call start to refresh it before finishing")
		}
		if s.lastCompletion != nil {
			return s.lastCompletion, nil
		}
		return map[string]any{"status": "not-started", "agentControlReleased": nil, "verification": "no-local-task", "scope": "No release was observed; use control_status to inspect browser spaces"}, nil
	}
	keepExpr, err := parseKeep(keep)
	if err != nil {
		return nil, err
	}
	value, err := runStandalone(completionSource(strconv.Itoa(id), keepExpr), 90*time.Second)
	if err != nil {
		return nil, err
	}
	result, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("invalid finish receipt; query control_status before retrying")
	}
	if _, present := result["agentControlReleased"]; !present {
		return nil, errors.New("finish receipt omitted release status; query control_status before retrying")
	}
	resultID, ok := result["spaceId"].(float64)
	status, _ := result["status"].(string)
	if !ok || int(resultID) != id || (status != "finished" && status != "already-released" && status != "release-unverified") {
		return nil, errors.New("invalid finish receipt; query control_status before retrying")
	}
	s.lastCompletion = result
	if !explicit || id == s.spaceID {
		s.clearCompletedState()
	}
	return result, nil
}
func (s *server) controlStatus(rawID string) (any, error) {
	id := s.spaceID
	if rawID != "" {
		var err error
		id, err = s.resolveSpace(rawID)
		if err != nil {
			return nil, err
		}
	} else if id == 0 && s.lastCompletion != nil {
		if v, ok := s.lastCompletion["spaceId"].(float64); ok {
			id = int(v)
		}
	}
	body := fmt.Sprintf(`const spaces=await listTaskSpaces();const current=spaces.find(s=>s.id===%d);return {spaceId:%d||null,ownership:current?.ownership||'closed',agentControlReleased:%d?(!current||current.ownership==='user'):null,agentSpaces:spaces.filter(s=>s.ownership==='agent').map(s=>({id:s.id,name:s.name})),scope:'Registry status only. Other agents may still have active spaces; do not finish them without explicit authorization.'};`, id, id, id)
	return runStandalone(body, 30*time.Second)
}
func (s *server) clearCompletedState() {
	s.taskName = ""
	s.spaceID = 0
	s.page = ""
	s.lastTabs = nil
	s.ownership = ""
}
