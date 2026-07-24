package main

const (
	vkCapital  = 0x14
	vkLControl = 0xA2
	vkEscape   = 0x1B
)

// keyAction describes a single synthetic key event to inject.
type keyAction struct {
	vk    uint16
	keyUp bool
}

// capsDown and lctrlSent track the dual-role Caps Lock state machine.
var capsDown, lctrlSent bool

// step processes one physical (non-sentinel) keyboard event and returns:
//
//   - suppress: true means swallow the original event.
//   - pre: synthetic keys to inject before re-injecting the original.
//
// When suppress is true and vkCode != vkCapital, the caller must also
// re-inject the original event (with sentinel) after executing pre, so
// that it arrives in the input stream after LCtrl has been established.
func step(vkCode uint32, isDown bool) (suppress bool, pre []keyAction) {
	if vkCode == vkCapital {
		if isDown && !capsDown {
			capsDown = true
			lctrlSent = false
		} else if !isDown && capsDown {
			if lctrlSent {
				pre = []keyAction{{vkLControl, true}}
			} else {
				pre = []keyAction{{vkEscape, false}, {vkEscape, true}}
			}
			capsDown = false
			lctrlSent = false
		}
		return true, pre
	}

	if capsDown {
		if !lctrlSent {
			pre = []keyAction{{vkLControl, false}}
			lctrlSent = true
		}
		return true, pre
	}

	return false, nil
}
