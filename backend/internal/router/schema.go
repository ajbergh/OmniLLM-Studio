package router

import "encoding/json"

const SchemaVersion = "router.v1"

func JSONSchemaResponseFormat() map[string]interface{} {
	return map[string]interface{}{
		"type": "json_schema",
		"json_schema": map[string]interface{}{
			"name":   "omnillm_router_decision",
			"strict": true,
			"schema": decisionSchema(),
		},
	}
}

func JSONObjectResponseFormat() map[string]interface{} {
	return map[string]interface{}{"type": "json_object"}
}

func SchemaJSON() string {
	b, _ := json.Marshal(decisionSchema())
	return string(b)
}

func decisionSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"route", "confidence", "requires_generation_llm", "rewritten_query", "clarifying_question", "reason", "sports"},
		"properties": map[string]interface{}{
			"route": map[string]interface{}{
				"type": "string",
				"enum": []string{
					string(RouteNone), string(RouteNormalLLM), string(RouteClarify), string(RouteSportsLookup),
					string(RouteFileSearch), string(RouteURLContext), string(RouteWebSearch), string(RouteBrowser),
					string(RouteRAG), string(RouteImageGeneration), string(RouteMusicGeneration), string(RouteArtifactGeneration),
				},
			},
			"confidence":              map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
			"requires_generation_llm": map[string]interface{}{"type": "boolean"},
			"rewritten_query":         map[string]interface{}{"type": []string{"string", "null"}},
			"clarifying_question":     map[string]interface{}{"type": []string{"string", "null"}},
			"reason":                  map[string]interface{}{"type": []string{"string", "null"}},
			"sports": map[string]interface{}{
				"type":                 []string{"object", "null"},
				"additionalProperties": false,
				"required": []string{
					"intent", "league", "sport", "team_query", "athlete_query", "second_athlete_query",
					"metric", "date", "date_label", "season", "limit", "game_detail_subtype",
				},
				"properties": map[string]interface{}{
					"intent":               map[string]interface{}{"type": []string{"string", "null"}},
					"league":               map[string]interface{}{"type": []string{"string", "null"}},
					"sport":                map[string]interface{}{"type": []string{"string", "null"}},
					"team_query":           map[string]interface{}{"type": []string{"string", "null"}},
					"athlete_query":        map[string]interface{}{"type": []string{"string", "null"}},
					"second_athlete_query": map[string]interface{}{"type": []string{"string", "null"}},
					"metric":               map[string]interface{}{"type": []string{"string", "null"}},
					"date":                 map[string]interface{}{"type": []string{"string", "null"}},
					"date_label":           map[string]interface{}{"type": []string{"string", "null"}},
					"season":               map[string]interface{}{"type": []string{"integer", "null"}},
					"limit":                map[string]interface{}{"type": []string{"integer", "null"}},
					"game_detail_subtype":  map[string]interface{}{"type": []string{"string", "null"}},
				},
			},
		},
	}
}
