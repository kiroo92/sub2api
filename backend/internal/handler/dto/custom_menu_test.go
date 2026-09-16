package dto

import (
	"encoding/json"
	"testing"
)

func TestCustomMenuOpenModeRoundTrip(t *testing.T) {
	items := ParseUserVisibleMenuItems(`[{"id":"legacy","visibility":"user"},{"id":"image","visibility":"user","open_mode":"new_tab","hide_open_button":true},{"id":"admin","visibility":"admin","open_mode":"new_tab"}]`)
	if len(items) != 2 || items[0].OpenMode != "" || items[0].HideOpenButton || items[1].OpenMode != "new_tab" || !items[1].HideOpenButton {
		t.Fatalf("unexpected visible menus: %+v", items)
	}
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	if parsed := ParseCustomMenuItems(string(raw)); len(parsed) != 2 || parsed[1].OpenMode != "new_tab" || !parsed[1].HideOpenButton {
		t.Fatalf("menu options lost during serialization: %s", raw)
	}
}
