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
func (s *server) completeSpace(rawID, keep string) (any, error) {
	id := s.spaceID
	explicit := strings.TrimSpace(rawID) != ""
	if explicit {
		var err error
		id, err = intArg("spaceId", rawID, 1, 1<<31-1)
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
		id, err = intArg("spaceId", rawID, 1, 1<<31-1)
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
