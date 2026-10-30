package emu

// BranchTargetMode is effective landing-check state, not CPU feature discovery.
// Only explicitly captured non-enforcement is currently modeled. Guarded-page
// enforcement and unspecified state fail closed rather than silently ignoring BTI.
type BranchTargetMode uint8

const (
	BranchTargetUnspecified BranchTargetMode = iota
	BranchTargetNonEnforced
)

func btiAllowed(word uint32, mode BranchTargetMode) bool {
	if mode != BranchTargetNonEnforced {
		return false
	}
	switch word {
	case 0xd503241f, 0xd503245f, 0xd503249f, 0xd50324df:
		return true
	}
	return false
}
