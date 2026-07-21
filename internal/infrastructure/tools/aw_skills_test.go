package tools

import (
	"context"
	"strings"
	"testing"
)

func TestSkillActionsRegisterAndDispatch(t *testing.T) {
	ctx := context.Background()
	ws := &workspace{
		root: t.TempDir(),
		skillCatalogFn: func(context.Context) (any, error) {
			return []map[string]any{{"id": "gmail-web", "description": "operar Gmail"}}, nil
		},
		skillReadFn: func(_ context.Context, id, path string) (any, error) {
			return map[string]any{"id": id, "path": path, "content": "FULL BODY"}, nil
		},
	}
	reg := ws.awRegistry()

	if _, ok := reg["skill.list"]; !ok {
		t.Fatal("skill.list not registered when SkillCatalogFn is set")
	}
	if _, ok := reg["skill.read"]; !ok {
		t.Fatal("skill.read not registered when SkillReadFn is set")
	}

	out, err := reg["skill.list"](ctx, map[string]any{}, ws)
	if err != nil || !strings.Contains(out, "gmail-web") {
		t.Fatalf("skill.list = %q, %v", out, err)
	}

	out, err = reg["skill.read"](ctx, map[string]any{"id": "gmail-web"}, ws)
	if err != nil || !strings.Contains(out, "FULL BODY") {
		t.Fatalf("skill.read = %q, %v", out, err)
	}

	// id is required.
	if _, err := reg["skill.read"](ctx, map[string]any{}, ws); err == nil {
		t.Fatal("skill.read without id should error")
	}
}

func TestSkillActionsAbsentWithoutFuncs(t *testing.T) {
	ws := &workspace{root: t.TempDir(), selfManage: true}
	reg := ws.awRegistry()
	if _, ok := reg["skill.read"]; ok {
		t.Fatal("skill.read must not register without SkillReadFn")
	}
	if _, ok := reg["skill.save"]; ok {
		t.Fatal("skill.save must not register without SkillManage")
	}
}

func TestSkillManageActionsRegisterAndDispatch(t *testing.T) {
	ctx := context.Background()
	var (
		gotSaveID    string
		gotSaveFiles []SkillFileInput
		gotEnabled   bool
		gotCreateEn  bool
	)
	ok := func(extra map[string]any) (any, error) {
		resp := map[string]any{"success": true, "contextRefreshed": true, "restartRequired": false}
		for k, v := range extra {
			resp[k] = v
		}
		return resp, nil
	}
	ws := &workspace{
		root: t.TempDir(),
		skillManage: &SkillManage{
			Detail: func(_ context.Context, id string) (any, error) {
				return map[string]any{"skill": map[string]any{"id": id}}, nil
			},
			Save: func(_ context.Context, id string, files []SkillFileInput) (any, error) {
				gotSaveID, gotSaveFiles = id, files
				return ok(nil)
			},
			Create: func(_ context.Context, _, _, _ string, enabled bool, _ []SkillFileInput) (any, error) {
				gotCreateEn = enabled
				return ok(nil)
			},
			SetEnabled: func(_ context.Context, _ string, enabled bool) (any, error) {
				gotEnabled = enabled
				return ok(nil)
			},
			ImportFile:   func(_ context.Context, _ string) (any, error) { return ok(nil) },
			ImportFolder: func(_ context.Context, _ string) (any, error) { return ok(nil) },
			Delete:       func(_ context.Context, _ string) (any, error) { return ok(nil) },
			Reset:        func(_ context.Context, _ string) (any, error) { return ok(nil) },
		},
	}
	reg := ws.awRegistry()

	for _, name := range []string{"skill.detail", "skill.save", "skill.create", "skill.set_enabled", "skill.import_file", "skill.import_folder", "skill.delete", "skill.reset"} {
		if _, found := reg[name]; !found {
			t.Fatalf("%s not registered", name)
		}
	}

	// detail requires id
	if _, err := reg["skill.detail"](ctx, map[string]any{}, ws); err == nil {
		t.Fatal("skill.detail without id should error")
	}

	// save passes id + files; response carries contextRefreshed
	out, err := reg["skill.save"](ctx, map[string]any{
		"id":    "gmail-web",
		"files": []any{map[string]any{"path": "SKILL.md", "content": "x"}},
	}, ws)
	if err != nil {
		t.Fatalf("skill.save error = %v", err)
	}
	if gotSaveID != "gmail-web" || len(gotSaveFiles) != 1 || gotSaveFiles[0].Path != "SKILL.md" {
		t.Fatalf("save callback got id=%q files=%+v", gotSaveID, gotSaveFiles)
	}
	if !strings.Contains(out, "contextRefreshed") {
		t.Fatalf("mutation response must include contextRefreshed: %s", out)
	}
	// save requires files
	if _, err := reg["skill.save"](ctx, map[string]any{"id": "x"}, ws); err == nil {
		t.Fatal("skill.save without files should error")
	}

	// create defaults enabled=true
	if _, err := reg["skill.create"](ctx, map[string]any{"id": "n", "files": []any{map[string]any{"path": "SKILL.md", "content": "x"}}}, ws); err != nil {
		t.Fatalf("skill.create error = %v", err)
	}
	if !gotCreateEn {
		t.Fatal("create enabled should default true")
	}

	// set_enabled requires the boolean
	if _, err := reg["skill.set_enabled"](ctx, map[string]any{"id": "x"}, ws); err == nil {
		t.Fatal("skill.set_enabled without enabled should error")
	}
	if _, err := reg["skill.set_enabled"](ctx, map[string]any{"id": "x", "enabled": false}, ws); err != nil {
		t.Fatalf("skill.set_enabled error = %v", err)
	}
	if gotEnabled {
		t.Fatal("set_enabled should pass false through")
	}

	// import + delete + reset require their path/id
	if _, err := reg["skill.import_file"](ctx, map[string]any{}, ws); err == nil {
		t.Fatal("skill.import_file without path should error")
	}
	if _, err := reg["skill.import_folder"](ctx, map[string]any{}, ws); err == nil {
		t.Fatal("skill.import_folder without path should error")
	}
	if _, err := reg["skill.delete"](ctx, map[string]any{}, ws); err == nil {
		t.Fatal("skill.delete without id should error")
	}
	if _, err := reg["skill.reset"](ctx, map[string]any{}, ws); err == nil {
		t.Fatal("skill.reset without id should error")
	}
}
