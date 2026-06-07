package annotation

// InputKind describes the input widget used by the wizard for a given annotation.
type InputKind int

const (
	KindText   InputKind = iota // free-text input
	KindChoice                  // select from a fixed list
	KindBool                    // yes/no (annotation is present or absent)
)

// AnnotationType describes a single annotation key supported by the wizard.
// Implementations live in registry.go. All methods are pure — no I/O.
type AnnotationType interface {
	Key()            string    // e.g. "when"
	Label()          string    // e.g. "When"  (shown in menu)
	Description()    string    // one-line explanation shown before prompting
	Kind()           InputKind
	Options()        []string  // non-nil only for KindChoice
	Validate(string) error     // called on non-empty user input; nil = accept
	Format(string)   string    // returns formatted comment line, e.g. "# @when(os=macos)"
}
