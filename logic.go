package main

const (
	vkCapital  = 0x14
	vkLControl = 0xA2
	vkEscape   = 0x1B
	vkReturn   = 0x0D
	vkRControl = 0xA3
)

// keyAction describes a single synthetic key event to inject.
type keyAction struct {
	vk    uint16
	keyUp bool
}

// dualKey is a key with tap vs. hold behavior:
//   tap  → emit tapVk down+up
//   held → inject modVk as a modifier while other keys are pressed
//
// useGrace: if true, the tap is deferred through the grace period so that
// a chord key arriving just after release is handled as modVk+key.
type dualKey struct {
	vk       uint32
	tapVk    uint16
	modVk    uint16
	useGrace bool
	down     bool
	modSent  bool
}

var dualKeys = []*dualKey{
	{vk: vkCapital, tapVk: vkEscape, modVk: vkLControl, useGrace: true},
	{vk: vkReturn, tapVk: vkReturn, modVk: vkRControl},
}

// isDualRoleKey reports whether vkCode is managed by the dualKeys table.
func isDualRoleKey(vkCode uint32) bool {
	for _, dk := range dualKeys {
		if dk.vk == vkCode {
			return true
		}
	}
	return false
}

// step processes one physical (non-sentinel) keyboard event and returns:
//
//   - suppress: true means swallow the original event.
//   - pre: synthetic keys to inject before re-injecting the original.
//   - grace: true means pre is a tap sequence that should be deferred
//     through the grace period rather than injected immediately.
//
// When suppress is true and !isDualRoleKey(vkCode), the caller must also
// re-inject the original event (with sentinel) after executing pre.
func step(vkCode uint32, isDown bool) (suppress bool, pre []keyAction, grace bool) {
	// Handle the dual-role key itself.
	for _, dk := range dualKeys {
		if vkCode != dk.vk {
			continue
		}
		if isDown && !dk.down {
			dk.down = true
			dk.modSent = false
		} else if !isDown && dk.down {
			if dk.modSent {
				pre = []keyAction{{dk.modVk, true}}
			} else {
				pre = []keyAction{{dk.tapVk, false}, {dk.tapVk, true}}
				grace = dk.useGrace
			}
			dk.down = false
			dk.modSent = false
		}
		return true, pre, grace
	}

	// Chord key: inject the modifier for the first held dual-role key.
	for _, dk := range dualKeys {
		if dk.down {
			if !dk.modSent {
				pre = []keyAction{{dk.modVk, false}}
				dk.modSent = true
			}
			return true, pre, false
		}
	}

	return false, nil, false
}
