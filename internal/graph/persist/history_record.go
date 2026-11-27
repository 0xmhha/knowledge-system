package persist

// HistoryRecord is the manifest's account of the git history a build read.
//   - Commits: commits walked (HEAD's history limited to the source's file
//     types, with git's history simplification); Tracked: the node kinds
//     whose line history was followed.
//   - Recorded: walked commits with a Commit node, by RecordedRule;
//     Unreachable: Commit nodes of commits no ref reaches (AMBIGUOUS).
//   - DepthPerNode: the cap on changed_in edges per node (newest first), 0
//     when every commit is recorded.
//   - Renames: how renamed files are followed.
//   - Boundaries: shallow-clone commits where the history stops.
//   - HunkWindow: the commits whose patches were stored as Hunk nodes.
//   - NumberSpaces: which repository a pull request number belongs to, and
//     the rule that decides it.
type HistoryRecord struct {
	Commits      int            `json:"commits"`
	Recorded     int            `json:"recorded"`
	Unreachable  int            `json:"unreachable"`
	RecordedRule string         `json:"recorded_rule"`
	Tracked      []string       `json:"tracked"`
	DepthPerNode int            `json:"depth_per_node"`
	Renames      string         `json:"renames"`
	Boundaries   []string       `json:"boundaries,omitempty"`
	HunkWindow   HunkWindow     `json:"hunk_window"`
	NumberSpaces NumberSpaceDoc `json:"number_spaces"`
}

// HunkWindow describes the commits whose hunks a build stored: the newest
// Limit non-merge commits of HEAD; Commits of them touched files in scope,
// from Oldest to Newest (SHA and author time, unix seconds).
type HunkWindow struct {
	Limit      int    `json:"limit"`
	Commits    int    `json:"commits"`
	Oldest     string `json:"oldest,omitempty"`
	OldestTime int64  `json:"oldest_time,omitempty"`
	Newest     string `json:"newest,omitempty"`
	NewestTime int64  `json:"newest_time,omitempty"`
}

// NumberSpaceDoc records where pull request numbers come from: Repo for the
// build's own commits, UpstreamRepo for commits reachable from UpstreamBase,
// and Rule, the decision in words.
type NumberSpaceDoc struct {
	Repo         string `json:"repo,omitempty"`
	UpstreamRepo string `json:"upstream_repo,omitempty"`
	UpstreamBase string `json:"upstream_base,omitempty"`
	Rule         string `json:"rule"`
}
