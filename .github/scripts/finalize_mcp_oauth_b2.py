from pathlib import Path

path = Path("backend/internal/mcpclient/oauth_test.go")
source = path.read_text()
if '"context"' not in source:
    source = source.replace('import (\n', 'import (\n\t"context"\n', 1)
path.write_text(source)
