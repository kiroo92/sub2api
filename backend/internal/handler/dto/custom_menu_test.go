package dto

import (
	"encoding/json"
	"testing"
)

func TestCustomMenuOpenModeRoundTrip(t *testing.T) {
	items := ParseUserVisibleMenuItems(`[{"id":"legacy","visibility":"user"},{"id":"image","visibility":"user","open_mode":"new_tab"},{"id":"admin","visibility":"admin","open_mode":"new_tab"}]`)
	if len(items) != 2 || items[0].OpenMode != "" || items[1].OpenMode != "new_tab" {
		t.Fatalf("unexpected visible menus: %+v", items)
	}
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	if parsed := ParseCustomMenuItems(string(raw)); len(parsed) != 2 || parsed[1].OpenMode != "new_tab" {
		t.Fatalf("open mode lost during serialization: %s", raw)
	}
}
