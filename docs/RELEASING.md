# リリース手順

LKAL のタグ付きリリースは、GitHub Actions の **Release** ワークフローだけで作ります。手元から `git tag` したり `git push --tags` したりしないでください。この文書の手順も、タグや GitHub Release を自動では作りません。

## オーナーがやること

1. 出したい版の変更が `main` に入っていることを確認する。版の番号は semver で、タグは `v` から始める。例: `v0.2.0`。プレリリースは `v0.2.0-rc.1` の形にする。
2. `CHANGELOG.md` のその版の見出しが、公開したい内容になっていることを確認する。
3. GitHub の **Actions** → **Release** → **Run workflow** を開く。
4. **Use workflow from** を `main` にする。
5. **version** にタグを入れる。例: `v0.2.0`。
6. **Run workflow** を押す。

ワークフローは次を順に行います。

1. `version` が `vMAJOR.MINOR.PATCH`（任意のプレリリースとビルドメタデータ付き）であることを検査する。違う文字列は失敗する。
2. 同じ名前のタグが既に無いことを確認する。
3. `main` の先端にその注釈付きタグを作り、`origin` へ push する。`main` 以外のブランチから起動した場合はタグを作らず失敗する。
4. タグの push が同じワークフローのリリースジョブを起動する。
5. GoReleaser が `lkal` を linux / darwin / windows の amd64 と arm64 向けにビルドし、アーカイブと `checksums.txt` を GitHub Release に載せる。バイナリの版は `-X main.version=<tag>` で埋め込む。`lkal version` は `v0.2.0` のようなタグ文字列を出す。

リリースの成果物は、リポジトリの **Releases** ページに出ます。`lkal version` はそのタグの版を出します。

## ローカルでの確認

タグは打ちません。設定の検査と、公開しないスナップショットビルドだけを行います。

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=publish
```

`dist/` は生成物なのでコミットしません。
