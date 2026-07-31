package style

import (
	"testing"
)

func TestStylePatchReset(t *testing.T) {
	tests := []struct {
		name       string
		base       Style
		patch      Style
		wantFG     Color
		wantFGSet  bool
		wantSubMod Modifier
	}{
		{
			name:       "patching with ResetStyle forces color reset and sub ModAll",
			base:       New().WithFG(Red).WithAddModifier(ModBold),
			patch:      ResetStyle(),
			wantFG:     Reset,
			wantFGSet:  true,
			wantSubMod: ModAll,
		},
		{
			name:       "patching ResetStyle with new FG updates FG and keeps BG Reset",
			base:       ResetStyle(),
			patch:      New().WithFG(Blue),
			wantFG:     Blue,
			wantFGSet:  true,
			wantSubMod: ModAll,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.base.Patch(tt.patch)
			fg, hasFG := got.Foreground()
			if hasFG != tt.wantFGSet {
				t.Errorf("hasFG = %v, want %v", hasFG, tt.wantFGSet)
			}
			if fg != tt.wantFG {
				t.Errorf("fg = %v, want %v", fg, tt.wantFG)
			}
			if got.SubModifier != tt.wantSubMod {
				t.Errorf("subModifier = %v, want %v", got.SubModifier, tt.wantSubMod)
			}
		})
	}
}

func TestAddSubModifierPrecedence(t *testing.T) {
	t.Run("WithAddModifier clears SubModifier flags", func(t *testing.T) {
		s := New().WithRemoveModifier(ModBold).WithAddModifier(ModBold)
		if s.AddModifier.Contains(ModBold) != true {
			t.Errorf("expected AddModifier to contain ModBold")
		}
		if s.SubModifier.Contains(ModBold) == true {
			t.Errorf("expected SubModifier to NOT contain ModBold")
		}
	})

	t.Run("WithRemoveModifier clears AddModifier flags", func(t *testing.T) {
		s := New().WithAddModifier(ModBold).WithRemoveModifier(ModBold)
		if s.AddModifier.Contains(ModBold) == true {
			t.Errorf("expected AddModifier to NOT contain ModBold")
		}
		if s.SubModifier.Contains(ModBold) != true {
			t.Errorf("expected SubModifier to contain ModBold")
		}
	})

	t.Run("ApplyModifiers order: insert add then remove sub", func(t *testing.T) {
		// s adds Bold and removes Italic
		s := FromModifiers(ModBold, ModItalic)
		base := ModItalic | ModDim
		got := s.ApplyModifiers(base)
		want := ModBold | ModDim
		if got != want {
			t.Errorf("ApplyModifiers = %v, want %v", got, want)
		}

		// Conflict: s adds Bold AND removes Bold (sub wins)
		sConflict := Style{AddModifier: ModBold, SubModifier: ModBold}
		gotConflict := sConflict.ApplyModifiers(ModBold)
		if gotConflict.Contains(ModBold) {
			t.Errorf("expected SubModifier to override AddModifier in ApplyModifiers")
		}
	})

	t.Run("Patch modifier precedence equation", func(t *testing.T) {
		s1 := FromModifier(ModBold)
		s2 := New().WithRemoveModifier(ModBold).WithAddModifier(ModItalic)
		patched := s1.Patch(s2)

		if patched.AddModifier.Contains(ModBold) {
			t.Errorf("patched AddModifier should not contain ModBold")
		}
		if !patched.AddModifier.Contains(ModItalic) {
			t.Errorf("patched AddModifier should contain ModItalic")
		}
		if !patched.SubModifier.Contains(ModBold) {
			t.Errorf("patched SubModifier should contain ModBold")
		}
	})
}
