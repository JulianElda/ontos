package ontos

import "strings"

// conflictMarkers are the substrings sync clients put in a filename when two
// machines changed one file and neither copy could be discarded. The store is
// synced, so these land next to the entries, where loading them would put two
// versions of one entry in play. Copied from pragma, which faces the same.
//
// Only unambiguous markers are here. OneDrive appends the machine name
// (x-DESKTOP-4K2J1.json) and Google Drive appends a counter (x (1).json); an
// entry id never has either shape, but a subject name could, so matching them
// would hide real subjects. Those are left to be noticed by eye.
var conflictMarkers = []string{
	"conflicted copy", // Dropbox, Nextcloud, ownCloud
	".sync-conflict-", // Syncthing
	"_conflict-",      // Seafile
	"(case conflict)", // Dropbox, on a case-insensitive filesystem
}

// IsConflictCopy reports whether a filename is a sync client's conflict copy
// rather than something ontos wrote.
func IsConflictCopy(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range conflictMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
