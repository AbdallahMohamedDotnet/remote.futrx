package githistory

type Repository struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	RelativePath string   `json:"relativePath"`
	CurrentRef   string   `json:"currentRef"`
	CurrentSHA   string   `json:"currentSha"`
	Dirty        bool     `json:"dirty"`
	DirtyFiles   []string `json:"dirtyFiles,omitempty"`
}

type Commit struct {
	SHA         string `json:"sha"`
	ShortSHA    string `json:"shortSha"`
	Subject     string `json:"subject"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	AuthorDate  int64  `json:"authorDate"`
	IsHead      bool   `json:"isHead"`
}

type Repositories struct {
	WorkspaceRoot string       `json:"workspaceRoot"`
	Repos         []Repository `json:"repos"`
}

type Commits struct {
	Repo    Repository `json:"repo"`
	Commits []Commit   `json:"commits"`
}

type Diff struct {
	Repo      Repository `json:"repo"`
	Commit    Commit     `json:"commit"`
	Diff      string     `json:"diff"`
	Truncated bool       `json:"truncated"`
}

type CheckoutInput struct {
	Repo              string `json:"repo"`
	SHA               string `json:"sha"`
	CheckpointMessage string `json:"checkpointMessage"`
}

type Checkout struct {
	Repo          Repository `json:"repo"`
	Output        string     `json:"output,omitempty"`
	CheckpointSHA string     `json:"checkpointSha,omitempty"`
}
