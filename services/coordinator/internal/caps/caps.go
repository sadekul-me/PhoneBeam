package caps

import "sort"

const (
	ScreenRead   = "screen.read"
	InputControl = "input.control"
	AudioRead    = "audio.read"
)

func Known() []string {
	return []string{ScreenRead, InputControl, AudioRead}
}

func IsKnown(name string) bool {
	switch name {
	case ScreenRead, InputControl, AudioRead:
		return true
	default:
		return false
	}
}

func Normalize(in []string) ([]string, bool) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, name := range in {
		if !IsKnown(name) {
			return nil, false
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, true
}

func Contains(set []string, name string) bool {
	for _, item := range set {
		if item == name {
			return true
		}
	}
	return false
}

// Intersect is requested ∩ granted ∩ device. Order is sorted for stable SAS and JSON.
func Intersect(requested, granted, device []string) []string {
	need := index(requested)
	haveGrant := index(granted)
	haveDevice := index(device)
	out := make([]string, 0, 3)
	for _, name := range Known() {
		if need[name] && haveGrant[name] && haveDevice[name] {
			out = append(out, name)
		}
	}
	return out
}

func Subset(inner, outer []string) bool {
	have := index(outer)
	for _, name := range inner {
		if !have[name] {
			return false
		}
	}
	return true
}

func index(in []string) map[string]bool {
	out := map[string]bool{}
	for _, name := range in {
		out[name] = true
	}
	return out
}
