package main

import (
	"reflect"
	"testing"
)

func resetState() {
	capsDown = false
	lctrlSent = false
}

// TestTap: press and release Caps Lock without any other key → Escape.
func TestTap(t *testing.T) {
	resetState()

	suppress, pre := step(vkCapital, true)
	if !suppress || len(pre) != 0 {
		t.Fatalf("caps down: want (true, []), got (%v, %v)", suppress, pre)
	}

	suppress, pre = step(vkCapital, false)
	want := []keyAction{{vkEscape, false}, {vkEscape, true}}
	if !suppress || !reflect.DeepEqual(pre, want) {
		t.Fatalf("caps up (tap): want (true, %v), got (%v, %v)", want, suppress, pre)
	}
}

// TestHeld: hold Caps Lock, press another key, release both → Ctrl, no Escape.
func TestHeld(t *testing.T) {
	resetState()

	step(vkCapital, true)

	// First non-Caps key: LCtrl must be injected before it.
	suppress, pre := step(0x43, true) // C down
	if !suppress {
		t.Fatal("C down: expected suppress=true")
	}
	if want := []keyAction{{vkLControl, false}}; !reflect.DeepEqual(pre, want) {
		t.Fatalf("C down: want pre=%v, got %v", want, pre)
	}

	// Key-up of same key: still suppressed, no additional pre-actions.
	suppress, pre = step(0x43, false) // C up
	if !suppress || len(pre) != 0 {
		t.Fatalf("C up: want (true, []), got (%v, %v)", suppress, pre)
	}

	// Caps up: inject LCtrl up only, no Escape.
	suppress, pre = step(vkCapital, false)
	if want := []keyAction{{vkLControl, true}}; !suppress || !reflect.DeepEqual(pre, want) {
		t.Fatalf("caps up (held): want (true, %v), got (%v, %v)", []keyAction{{vkLControl, true}}, suppress, pre)
	}
}

// TestNoDoubleEscapeWhenHeld: Escape must not fire when Caps was used as modifier.
func TestNoDoubleEscapeWhenHeld(t *testing.T) {
	resetState()
	step(vkCapital, true)
	step(0x43, true) // use Caps as modifier

	_, pre := step(vkCapital, false)
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

	_, pre := step(0x56, true) // V down, LCtrl already sent
	if len(pre) != 0 {
		t.Fatalf("second key: want no pre-actions, got %v", pre)
	}
}

// TestNormalKeyPassthrough: keys pressed without Caps held are not affected.
func TestNormalKeyPassthrough(t *testing.T) {
	resetState()

	suppress, pre := step(0x41, true) // A down
	if suppress || len(pre) != 0 {
		t.Fatalf("normal key: want (false, []), got (%v, %v)", suppress, pre)
	}
}

// TestCapsPrintDialog: Caps+P must produce LCtrl_Down → P_Down → P_Up → LCtrl_Up.
// This verifies the pre-action ordering that makes Ctrl+P (print) work.
// hookProc must call reInject(ks, !isDown) — passing isDown instead of !isDown
// re-injects P as a key-up on the down stroke, silently breaking Ctrl+P.
func TestCapsPrintDialog(t *testing.T) {
	resetState()

	step(vkCapital, true) // Caps down

	// P down: LCtrl_Down must precede the re-injected P_Down.
	suppress, pre := step(0x50, true)
	if !suppress {
		t.Fatal("P down: expected suppress=true")
	}
	if want := []keyAction{{vkLControl, false}}; !reflect.DeepEqual(pre, want) {
		t.Fatalf("P down: want pre=%v (LCtrl down), got %v", want, pre)
	}

	// P up: no pre-actions, still suppressed for re-injection.
	suppress, pre = step(0x50, false)
	if !suppress || len(pre) != 0 {
		t.Fatalf("P up: want (true, []), got (%v, %v)", suppress, pre)
	}

	// Caps up: LCtrl_Up only, no Escape.
	_, pre = step(vkCapital, false)
	if want := []keyAction{{vkLControl, true}}; !reflect.DeepEqual(pre, want) {
		t.Fatalf("caps up: want %v (LCtrl up), got %v", want, pre)
	}
}

// TestDoubleTap: two consecutive taps each produce exactly one Escape.
func TestDoubleTap(t *testing.T) {
	resetState()
	for i := range 2 {
		step(vkCapital, true)
		_, pre := step(vkCapital, false)
		want := []keyAction{{vkEscape, false}, {vkEscape, true}}
		if !reflect.DeepEqual(pre, want) {
			t.Fatalf("tap %d: want %v, got %v", i+1, want, pre)
		}
	}
}
