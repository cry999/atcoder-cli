package contests

type Family interface {
	ContestName() string
	BaseDir(workdir string) string
}
