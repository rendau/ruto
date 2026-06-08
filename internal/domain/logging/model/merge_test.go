package model

import "testing"

func TestMergeReplace(t *testing.T) {
	parent := Logging{Level: "all", Headers: true, ReqBody: true, ReqBodyLimit: 100}
	child := Logging{Mode: "replace", Level: "error", QueryParams: true}

	got := Merge(parent, child)

	if got.Level != "error" {
		t.Fatalf("level: want error, got %q", got.Level)
	}
	if got.Headers || got.ReqBody {
		t.Fatalf("replace must drop parent flags, got %+v", got)
	}
	if !got.QueryParams {
		t.Fatalf("replace must keep child flags, got %+v", got)
	}
	if got.ReqBodyLimit != 0 {
		t.Fatalf("replace must not inherit parent limit, got %d", got.ReqBodyLimit)
	}
}

func TestMergeExtendUnionsFlagsAndInherits(t *testing.T) {
	parent := Logging{Level: "all", Headers: true, ReqBodyLimit: 100, RespBodyLimit: 200}
	child := Logging{Mode: "extend", QueryParams: true, RespBodyLimit: 50}

	got := Merge(parent, child)

	if !got.Headers || !got.QueryParams {
		t.Fatalf("extend must union flags, got %+v", got)
	}
	if got.Level != "all" {
		t.Fatalf("extend must inherit parent level when child empty, got %q", got.Level)
	}
	if got.ReqBodyLimit != 100 {
		t.Fatalf("extend must inherit parent req limit when child zero, got %d", got.ReqBodyLimit)
	}
	if got.RespBodyLimit != 50 {
		t.Fatalf("extend must keep child resp limit when set, got %d", got.RespBodyLimit)
	}
}

func TestEffectiveLevelDefaultsToError(t *testing.T) {
	empty := Logging{}
	if empty.EffectiveLevel() != "error" {
		t.Fatalf("empty level must resolve to error")
	}
	all := Logging{Level: "all"}
	if all.EffectiveLevel() != "all" {
		t.Fatalf("explicit level must be preserved")
	}
	none := Logging{Level: "none"}
	if none.EffectiveLevel() != "none" {
		t.Fatalf("explicit none level must be preserved")
	}
}
