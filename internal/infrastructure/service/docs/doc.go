// Package docs implements lexical DocsIndex (Markdown walk + docs_index.json).
//
// Wire agent: call Reindex (or rely on NewIndex AutoBuild) then Gate at startup
// before serving AI turns. Storage files live under storageRoot:
// docs_index.json and docs_index.status.json.
package docs

// Defaults mirror laravel-aipedia web/config/webchat.php.
const (
	DefaultTopK         = 5
	DefaultMinScore     = 0.5
	DefaultFuzzyEnabled = true
	DefaultAppID        = "buatpostingan"
)
