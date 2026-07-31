package style

import (
	"encoding/json"
	"testing"
)

func TestColorJSONCurrentAndLegacyFormats(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  Color
	}{
		{`"bright-white"`, White},
		{`"#00FF00"`, RGB(0, 255, 0)},
		{`"42"`, Indexed(42)},
		{`{"Rgb":[255,0,255]}`, RGB(255, 0, 255)},
		{`{"Indexed":10}`, Indexed(10)},
	} {
		var got Color
		if err := json.Unmarshal([]byte(tc.input), &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("Unmarshal(%s) = %v, want %v", tc.input, got, tc.want)
		}
	}
	encoded, err := json.Marshal(RGB(255, 0, 255))
	if err != nil || string(encoded) != `"#FF00FF"` {
		t.Fatalf("Marshal RGB = %s, %v", encoded, err)
	}
}

func TestStyleJSONMatchesRatatuiSerde(t *testing.T) {
	value := Style{
		FG:                RGB(255, 0, 255),
		BG:                White,
		UnderlineColor:    Indexed(3),
		HasFG:             true,
		HasBG:             true,
		HasUnderlineColor: true,
		AddModifier:       ModUnderlined,
		SubModifier:       ModCrossedOut,
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var gotMap map[string]any
	if err := json.Unmarshal(encoded, &gotMap); err != nil {
		t.Fatal(err)
	}
	wantMap := map[string]any{
		"fg":              "#FF00FF",
		"bg":              "White",
		"underline_color": "3",
		"add_modifier":    "UNDERLINED",
		"sub_modifier":    "CROSSED_OUT",
	}
	if len(gotMap) != len(wantMap) {
		t.Fatalf("JSON = %s", encoded)
	}
	for key, want := range wantMap {
		if gotMap[key] != want {
			t.Fatalf("JSON[%q] = %#v, want %#v", key, gotMap[key], want)
		}
	}
	var roundtrip Style
	if err := json.Unmarshal(encoded, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if roundtrip != value {
		t.Fatalf("roundtrip = %#v, want %#v", roundtrip, value)
	}
}

func TestStyleJSONDefaultsAndNullModifiers(t *testing.T) {
	encoded, err := json.Marshal(Style{})
	if err != nil || string(encoded) != `{}` {
		t.Fatalf("Marshal zero style = %s, %v", encoded, err)
	}
	var got Style
	if err := json.Unmarshal([]byte(`{"add_modifier":null,"sub_modifier":null}`), &got); err != nil {
		t.Fatal(err)
	}
	if got != (Style{}) {
		t.Fatalf("null modifiers = %#v, want zero", got)
	}
}
