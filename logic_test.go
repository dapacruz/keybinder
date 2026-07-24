package main

import (
	"reflect"
	"testing"
)

func resetState() {
	for _, dk := range dualKeys {
		dk.down = false
		dk.modSent = false
	}
}

// ── Caps Lock tests ────────────────────────────────────────────────────────────

// TestTap: press and release Caps Lock without any other key → Escape.
func TestTap(t *testing.T) {
	resetState()

	suppress, pre, _ := step(vkCapital, true)
	if !suppress || len(pre) != 0 {
		t.Fatalf("caps down: want (true, []), got (%v, %v)", suppress, pre)
	}

	suppress, pre, grace := step(vkCapital, false)
	want := []keyAction{{vkEscape, false}, {vkEscape, true}}
	if !suppress || !grace || !reflect.DeepEqual(pre, want) {
		t.Fatalf("caps up (tap): want (true, %v, grace=true), got (%v, %v, %v)", want, suppress, pre, grace)
	}
}

// TestHeld: hold Caps Lock, press another key, release both → Ctrl, no Escape.
func TestHeld(t *testing.T) {
	resetState()

	step(vkCapital, true)

	suppress, pre, _ := step(0x43, true) // C down
	if !suppress {
		t.Fatal("C down: expected suppress=true")
	}
	if want := []keyAction{{vkLControl, false}}; !reflect.DeepEqual(pre, want) {
		t.Fatalf("C down: want pre=%v, got %v", want, pre)
	}

	suppress, pre, _ = step(0x43, false) // C up
	if !suppress || len(pre) != 0 {
		t.Fatalf("C up: want (true, []), got (%v, %v)", suppress, pre)
	}

	suppress, pre, _ = step(vkCapital, false)
	if want := []keyAction{{vkLControl, true}}; !suppress || !reflect.DeepEqual(pre, want) {
		t.Fatalf("caps up (held): want (true, %v), got (%v, %v)", []keyAction{{vkLControl, true}}, suppress, pre)
	}
}

// TestNoDoubleEscapeWhenHeld: Escape must not fire when Caps was used as modifier.
func TestNoDoubleEscapeWhenHeld(t *testing.T) {
	resetState()
	step(vkCapital, true)
	step(0x43, true)

	_, pre, _ := step(vkCapital, false)
	for _, a := range pre {
		if a.vk == vkEscape {
			t.Fatal("Escape must not fire when Caps Lock was used as a modifier")
		}
	}
}

// TestSecondKeyNoExtraCtrl: LCtrl should be injected only once per Caps hold.
func TestSecondKeyNoExtraCtrl(t *testing.T) {
	resetState()
	step(vkCapital, true)
	step(0x43, true) // C — triggers LCtrl injection

	_, pre, _ := step(0x56, true) // V down, LCtrl already sent
	if len(pre) != 0 {
		t.Fatalf("second key: want no pre-actions, got %v", pre)
	}
}

// TestNormalKeyPassthrough: keys pressed without Caps held are not affected.
func TestNormalKeyPassthrough(t *testing.T) {
	resetState()

	suppress, pre, _ := step(0x41, true) // A down
	if suppress || len(pre) != 0 {
		t.Fatalf("normal key: want (false, []), got (%v, %v)", suppress, pre)
	}
}

// TestCapsPrintDialog: Caps+P must produce LCtrl_Down → P_Down → P_Up → LCtrl_Up.
func TestCapsPrintDialog(t *testing.T) {
	resetState()

	step(vkCapital, true)

	suppress, pre, _ := step(0x50, true)
	if !suppress {
		t.Fatal("P down: expected suppress=true")
	}
	if want := []keyAction{{vkLControl, false}}; !reflect.DeepEqual(pre, want) {
		t.Fatalf("P down: want pre=%v (LCtrl down), got %v", want, pre)
	}

	suppress, pre, _ = step(0x50, false)
	if !suppress || len(pre) != 0 {
		t.Fatalf("P up: want (true, []), got (%v, %v)", suppress, pre)
	}

	_, pre, _ = step(vkCapital, false)
	if want := []keyAction{{vkLControl, true}}; !reflect.DeepEqual(pre, want) {
		t.Fatalf("caps up: want %v (LCtrl up), got %v", want, pre)
	}
}

// TestDoubleTap: two consecutive taps each produce exactly one Escape.
func TestDoubleTap(t *testing.T) {
	resetState()
	for i := range 2 {
		step(vkCapital, true)
		_, pre, _ := step(vkCapital, false)
		want := []keyAction{{vkEscape, false}, {vkEscape, true}}
		if !reflect.DeepEqual(pre, want) {
			t.Fatalf("tap %d: want %v, got %v", i+1, want, pre)
		}
	}
}

// ── Enter tests ────────────────────────────────────────────────────────────────

// TestEnterTap: press and release Enter without any other key → Enter.
func TestEnterTap(t *testing.T) {
	resetState()

	suppress, pre, _ := step(vkReturn, true)
	if !suppress || len(pre) != 0 {
		t.Fatalf("enter down: want (true, []), got (%v, %v)", suppress, pre)
	}

	suppress, pre, grace := step(vkReturn, false)
	want := []keyAction{{vkReturn, false}, {vkReturn, true}}
	if !suppress || grace || !reflect.DeepEqual(pre, want) {
		t.Fatalf("enter up (tap): want (true, %v, grace=false), got (%v, %v, %v)", want, suppress, pre, grace)
	}
}

// TestEnterHeld: hold Enter, press another key → RCtrl modifier; no Enter on release.
func TestEnterHeld(t *testing.T) {
	resetState()

	step(vkReturn, true)

	suppress, pre, _ := step(0x4A, true) // J down
	if !suppress {
		t.Fatal("J down: expected suppress=true")
	}
	if want := []keyAction{{vkRControl, false}}; !reflect.DeepEqual(pre, want) {
		t.Fatalf("J down: want pre=%v, got %v", want, pre)
	}

	suppress, pre, _ = step(0x4A, false) // J up
	if !suppress || len(pre) != 0 {
		t.Fatalf("J up: want (true, []), got (%v, %v)", suppress, pre)
	}

	suppress, pre, _ = step(vkReturn, false)
	if want := []keyAction{{vkRControl, true}}; !suppress || !reflect.DeepEqual(pre, want) {
		t.Fatalf("enter up (held): want (true, %v), got (%v, %v)", want, suppress, pre)
	}
}

// TestEnterNoGrace: Enter tap must not request a grace period.
func TestEnterNoGrace(t *testing.T) {
	resetState()
	step(vkReturn, true)
	_, _, grace := step(vkReturn, false)
	if grace {
		t.Fatal("Enter tap must not use grace period")
	}
}

// TestEnterSecondKeyNoExtraRCtrl: RCtrl injected only once per Enter hold.
func TestEnterSecondKeyNoExtraRCtrl(t *testing.T) {
	resetState()
	step(vkReturn, true)
	step(0x4A, true) // J — triggers RCtrl injection

	_, pre, _ := step(0x4B, true) // K down, RCtrl already sent
	if len(pre) != 0 {
		t.Fatalf("second key: want no pre-actions, got %v", pre)
	}
}

// TestEnterNoEscapeWhenHeld: Enter held as modifier must not emit Enter on release.
func TestEnterNoEscapeWhenHeld(t *testing.T) {
	resetState()
	step(vkReturn, true)
	step(0x4A, true)

	_, pre, _ := step(vkReturn, false)
	for _, a := range pre {
		if a.vk == vkReturn && !a.keyUp {
			t.Fatal("Enter key-down must not fire when Enter was used as a modifier")
		}
	}
}
