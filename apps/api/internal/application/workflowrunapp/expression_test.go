package workflowrunapp

import (
	"testing"
	"time"

	"fluxa-api/internal/domain/workflowrun"
)

func TestExpressionServiceTeamAndTime(t *testing.T) {
	vars := map[string]any{
		"release": map[string]any{"services": []any{map[string]any{"key": "payments"}}},
		"project": map[string]any{"team": "core"},
	}
	p, err := compileBool(`hasService("payments") && inTeam("core")`)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := p.Eval(vars)
	if err != nil || !ok {
		t.Fatalf("got %v, %v", ok, err)
	}
	want := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	got, err := evalTimestamp(`"2026-07-11T20:00:00Z"`, vars)
	if err != nil || !got.Equal(want) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestValidateDefinitionRejectsMissingTarget(t *testing.T) {
	def := workflowrun.Definition{Name: "bad", Version: 1, Start: "check", Steps: map[string]workflowrun.Step{"check": {Type: "condition", Expression: "true", OnTrue: "missing", OnFalse: "missing"}}}
	if err := ValidateDefinition(def); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestDefaultDefinitionIsValid(t *testing.T) {
	if err := ValidateDefinition(workflowrun.DefaultReleaseDefinition()); err != nil {
		t.Fatal(err)
	}
}
