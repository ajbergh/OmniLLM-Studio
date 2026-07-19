# RAG Validation Diagnostic

go_test_status=1
race_test_status=1

## go test ./...
```text
go: downloading github.com/google/uuid v1.6.0
go: downloading github.com/wailsapp/wails/v2 v2.13.0
go: downloading golang.org/x/crypto v0.53.0
go: downloading github.com/go-rod/rod v0.116.2
go: downloading github.com/go-rod/stealth v0.4.9
go: downloading github.com/go-pdf/fpdf v0.9.0
go: downloading github.com/xuri/excelize/v2 v2.11.0
go: downloading gopkg.in/yaml.v3 v3.0.1
go: downloading modernc.org/sqlite v1.54.0
go: downloading github.com/ledongthuc/pdf v0.0.0-20250511090121-5959a4027728
go: downloading golang.org/x/net v0.56.0
go: downloading github.com/chinmaykhachane/espn-go v0.1.1
go: downloading github.com/go-chi/chi/v5 v5.3.1
go: downloading github.com/philippgille/chromem-go v0.7.0
go: downloading github.com/go-chi/cors v1.2.2
go: downloading codeberg.org/readeck/go-readability/v2 v2.1.2
go: downloading github.com/drumkitai/go-word v1.0.1
go: downloading github.com/ysmood/goob v0.4.0
go: downloading github.com/ysmood/got v0.40.0
go: downloading github.com/ysmood/gson v0.7.3
go: downloading github.com/ysmood/fetchup v0.2.3
go: downloading github.com/ysmood/leakless v0.9.0
go: downloading github.com/richardlehane/mscfb v1.0.7
go: downloading github.com/tiendc/go-deepcopy v1.7.2
go: downloading github.com/xuri/efp v0.0.1
go: downloading github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9
go: downloading golang.org/x/text v0.38.0
go: downloading github.com/yuin/goldmark v1.7.8
go: downloading github.com/litao91/goldmark-mathjax v0.0.0-20210217064022-a43cf739a50f
go: downloading github.com/leaanthony/u v1.1.1
go: downloading github.com/richardlehane/msoleps v1.0.6
go: downloading github.com/go-shiori/dom v0.0.0-20230515143342-73569d674e1c
go: downloading github.com/itlightning/dateparse v0.2.1
go: downloading github.com/leaanthony/slicer v1.6.0
go: downloading github.com/andybalholm/cascadia v1.3.3
go: downloading github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f
go: downloading github.com/leaanthony/go-ansi-parser v1.6.1
go: downloading github.com/pkg/errors v0.9.1
go: downloading github.com/rivo/uniseg v0.4.7
go: downloading golang.org/x/sys v0.46.0
go: downloading modernc.org/libc v1.74.1
go: downloading modernc.org/memory v1.11.0
go: downloading github.com/dustin/go-humanize v1.0.1
go: downloading modernc.org/mathutil v1.7.1
go: downloading github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec
# github.com/ajbergh/omnillm-studio/cmd/desktop
cmd/desktop/main.go:30:12: pattern all:frontend_dist: no matching files found
FAIL	github.com/ajbergh/omnillm-studio/cmd/desktop [setup failed]
# github.com/ajbergh/omnillm-studio/internal/repository
internal/repository/music_generation_repo.go:150:6: rowScanner redeclared in this block
	internal/repository/document_chunk.go:498:6: other declaration of rowScanner
internal/repository/rag_index.go:337:22: cannot use generationID (variable of type string) as *string value in argument to nullableString
internal/repository/rag_index.go:337:52: cannot use libraryFileID (variable of type string) as *string value in argument to nullableString
internal/repository/rag_index.go:337:83: cannot use attachmentID (variable of type string) as *string value in argument to nullableString
internal/repository/rag_index.go:379:6: nullableString redeclared in this block
	internal/repository/mcp.go:470:6: other declaration of nullableString
FAIL	github.com/ajbergh/omnillm-studio/cmd/playwrightseed [build failed]
FAIL	github.com/ajbergh/omnillm-studio/cmd/playwrightseedchat [build failed]
FAIL	github.com/ajbergh/omnillm-studio/cmd/server [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/agent [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/analytics [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/api [build failed]
?   	github.com/ajbergh/omnillm-studio/internal/apps	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/artifacts	0.010s
?   	github.com/ajbergh/omnillm-studio/internal/auth	[no test files]
FAIL	github.com/ajbergh/omnillm-studio/internal/browser [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/bundle [build failed]
?   	github.com/ajbergh/omnillm-studio/internal/config	[no test files]
?   	github.com/ajbergh/omnillm-studio/internal/crypto	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/db	0.054s
ok  	github.com/ajbergh/omnillm-studio/internal/document	0.008s
FAIL	github.com/ajbergh/omnillm-studio/internal/eval [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/filelibrary [build failed]
?   	github.com/ajbergh/omnillm-studio/internal/jobs	[no test files]
FAIL	github.com/ajbergh/omnillm-studio/internal/llm [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/mcpclient [build failed]
?   	github.com/ajbergh/omnillm-studio/internal/memory	[no test files]
ok  	github.com/ajbergh/omnillm-studio/internal/models	0.003s
FAIL	github.com/ajbergh/omnillm-studio/internal/music [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/plugins [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/rag [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/repository [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/router [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/search [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/sports [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/tasks [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/tasktools [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/templates [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/tools [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/urlcontext [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/video [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/websearch [build failed]
ok  	github.com/ajbergh/omnillm-studio/internal/wordgen	0.007s
FAIL
```

## focused race tests
```text
# github.com/ajbergh/omnillm-studio/internal/repository
internal/repository/music_generation_repo.go:150:6: rowScanner redeclared in this block
	internal/repository/document_chunk.go:498:6: other declaration of rowScanner
internal/repository/rag_index.go:337:22: cannot use generationID (variable of type string) as *string value in argument to nullableString
internal/repository/rag_index.go:337:52: cannot use libraryFileID (variable of type string) as *string value in argument to nullableString
internal/repository/rag_index.go:337:83: cannot use attachmentID (variable of type string) as *string value in argument to nullableString
internal/repository/rag_index.go:379:6: nullableString redeclared in this block
	internal/repository/mcp.go:470:6: other declaration of nullableString
FAIL	github.com/ajbergh/omnillm-studio/internal/rag [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/repository [build failed]
FAIL	github.com/ajbergh/omnillm-studio/internal/filelibrary [build failed]
ok  	github.com/ajbergh/omnillm-studio/internal/document	1.018s
FAIL
```
