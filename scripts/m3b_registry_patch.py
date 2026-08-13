from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    if old not in text:
        raise SystemExit(f"missing anchor in {path}: {old[:120]!r}")
    p.write_text(text.replace(old, new, 1), encoding="utf-8")

path = "backend/internal/tools/registry.go"
replace_once(path,
'''// resolution, and draft-to-ready transition are independent API gates and none
// implies Git push or another hosted gate.''',
'''// resolution, draft-to-ready transition, and guarded direct merge are independent
// API gates and none implies Git push or another hosted gate.''')
replace_once(path,
'''\t\t\tif remoteGitService.GitHubPullRequestReadyMutationEnabled() {
\t\t\t\tfor _, tool := range NewGitHubPullRequestReadyTools(remoteGitService) {
\t\t\t\t\tr.MustRegister(tool)
\t\t\t\t}
\t\t\t}
\t\t\tif remoteGitService.CloneMutationEnabled() {''',
'''\t\t\tif remoteGitService.GitHubPullRequestReadyMutationEnabled() {
\t\t\t\tfor _, tool := range NewGitHubPullRequestReadyTools(remoteGitService) {
\t\t\t\t\tr.MustRegister(tool)
\t\t\t\t}
\t\t\t}
\t\t\tif remoteGitService.GitHubPullRequestMergeMutationEnabled() {
\t\t\t\tfor _, tool := range NewGitHubPullRequestMergeTools(remoteGitService) {
\t\t\t\t\tr.MustRegister(tool)
\t\t\t\t}
\t\t\t}
\t\t\tif remoteGitService.CloneMutationEnabled() {''')

path = "backend/internal/tools/registry_github_credentials.go"
replace_once(path,
'''\t\tcase *githubPullRequestMergeEligibilityTool:
\t\t\ttyped.service = service
\t\t}
''',
'''\t\tcase *githubPullRequestMergeEligibilityTool:
\t\t\ttyped.service = service
\t\tcase *githubPullRequestMergeTool:
\t\t\ttyped.service = service
\t\t}
''')
