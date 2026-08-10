# Package Manager Publishing — Design Spec

## Overview

При пуше тега `v*` release workflow автоматически публикует `aide` во все 6 пакетных менеджеров: Homebrew, Scoop, Winget, Chocolatey, APT, DNF.

Нужно создать 2 новых репозитория и добавить 1 job в существующий `release.yml`.

## New Repositories

| Repo | Purpose | Type |
|------|---------|------|
| `accreation/homebrew-tap` | Homebrew formula | Git repo |
| `accreation/scoop-bucket` | Scoop manifest | Git repo |
| `accreation/aide-repo` | APT + DNF via GitHub Pages | Git repo with `gh-pages` enabled |

## GPG Key Management

Для APT/DNF-репозиториев нужен GPG-ключ. Генерируется один раз:

```bash
gpg --full-generate-key
# Name: Aide Package Signing
# Email: aide@accreation.com
# Type: RSA 4096, no expiry
```

- **Приватный ключ**: экспортируется в GitHub Secret `GPG_PRIVATE_KEY`
- **Публичный ключ**: `gpg.key` в корне `aide-repo` (для импорта пользователями)
- **Passphrase** (если есть): GitHub Secret `GPG_PASSPHRASE`

## Per-Manager Design

### 1. Homebrew Tap

**Repository**: `accreation/homebrew-tap`
**Formula path**: `Formula/aide.rb`

CI генерирует формулу из Go-шаблона, подставляя версию и SHA256 (из `checksums.txt` в релизе):

```ruby
class Aide < Formula
  desc "The package.json for your AI development environment"
  homepage "https://github.com/accreation/aide"
  version "$VERSION"
  license "AGPL-3.0"

  if OS.mac?
    if Hardware::CPU.arm?
      url "https://github.com/accreation/aide/releases/download/v$VERSION/aide-darwin-arm64.tar.gz"
      sha256 "$SHA_DARWIN_ARM64"
    else
      url "https://github.com/accreation/aide/releases/download/v$VERSION/aide-darwin-amd64.tar.gz"
      sha256 "$SHA_DARWIN_AMD64"
    end
  elsif OS.linux?
    if Hardware::CPU.arm?
      url "https://github.com/accreation/aide/releases/download/v$VERSION/aide-linux-arm64.tar.gz"
      sha256 "$SHA_LINUX_ARM64"
    else
      url "https://github.com/accreation/aide/releases/download/v$VERSION/aide-linux-amd64.tar.gz"
      sha256 "$SHA_LINUX_AMD64"
    end
  end

  def install
    bin.install Dir["aide-*"].first => "aide"
  end

  test do
    system "#{bin}/aide", "--version"
  end
end
```

**CI action**: clone `homebrew-tap` → render template → commit + push.

### 2. Scoop Bucket

**Repository**: `accreation/scoop-bucket`
**Manifest path**: `bucket/aide.json`

```json
{
  "version": "$VERSION",
  "description": "The package.json for your AI development environment",
  "homepage": "https://github.com/accreation/aide",
  "license": "AGPL-3.0",
  "architecture": {
    "64bit": {
      "url": "https://github.com/accreation/aide/releases/download/v$VERSION/aide-windows-amd64.exe.zip",
      "hash": "sha256:$SHA_WINDOWS_AMD64"
    },
    "arm64": {
      "url": "https://github.com/accreation/aide/releases/download/v$VERSION/aide-windows-arm64.exe.zip",
      "hash": "sha256:$SHA_WINDOWS_ARM64"
    }
  },
  "bin": [["aide-windows-amd64.exe", "aide"]],
  "checkver": "github",
  "autoupdate": {
    "architecture": {
      "64bit": {
        "url": "https://github.com/accreation/aide/releases/download/v$version/aide-windows-amd64.exe.zip"
      },
      "arm64": {
        "url": "https://github.com/accreation/aide/releases/download/v$version/aide-windows-arm64.exe.zip"
      }
    }
  }
}
```

**CI action**: clone `scoop-bucket` → render template → commit + push.

### 3. Winget

**Submission**: PR в `microsoft/winget-pkgs`
**PackageIdentifier**: `Accreation.Aide`
**Tool**: `wingetcreate` (dotnet tool)

**First submission** (ручная): создать пакет через `wingetcreate new`, PR принимают модераторы Microsoft (обычно 1-3 дня).

**Subsequent updates** (автоматическая в CI):
```bash
dotnet tool install --global Microsoft.WingetCreate
wingetcreate update Accreation.Aide \
  -v "$VERSION" \
  -u "https://github.com/accreation/aide/releases/download/v$VERSION/aide-windows-amd64.exe.zip" \
  -t "$WINGET_GITHUB_TOKEN" \
  --submit
```

**Required secret**: `WINGET_GITHUB_TOKEN` — GitHub PAT с правами на PR в публичные репо (нужен отдельный bot-аккаунт, т.к. GitHub запрещает PR в чужие репо через actions токен).

### 4. Chocolatey

**Package ID**: `aide`
**Tool**: `choco` CLI
**Required secret**: `CHOCOLATEY_API_KEY`

**Template files** (хранятся в `aide` репо в `packaging/chocolatey/`):

`aide.nuspec`:
```xml
<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2015/06/nuspec.xsd">
  <metadata>
    <id>aide</id>
    <version>$version$</version>
    <title>Aide</title>
    <authors>Accreation</authors>
    <projectUrl>https://github.com/accreation/aide</projectUrl>
    <license type="expression">AGPL-3.0</license>
    <description>The package.json for your AI development environment</description>
  </metadata>
  <files>
    <file src="tools\**" target="tools" />
  </files>
</package>
```

`tools/chocolateyInstall.ps1` (шаблон, `$version$`, `$url64$`, `$checksum64$` заменяются в CI):
```powershell
$ErrorActionPreference = 'Stop'
$packageName = 'aide'
$url64 = '$url64$'
$checksum64 = '$checksum64$'

$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$binDir = "$env:USERPROFILE\.local\bin"

New-Item -ItemType Directory -Force -Path $binDir | Out-Null

$packageArgs = @{
  packageName   = $packageName
  url64bit      = $url64
  checksum64    = $checksum64
  checksumType64 = 'sha256'
  unzipLocation = $binDir
}

Install-ChocolateyZipPackage @packageArgs

# Rename the extracted binary (aide-windows-amd64.exe -> aide.exe)
$extracted = Get-ChildItem "$binDir\aide-windows-*.exe" | Select-Object -First 1
Move-Item $extracted.FullName "$binDir\aide.exe" -Force

# Add to user PATH
Install-ChocolateyPath $binDir 'User'
```

**CI action**: 
1. Checkout `aide` repo
2. Render `nuspec` and `chocolateyInstall.ps1` with version + checksum from release
3. `choco pack`
4. `choco push --api-key $CHOCOLATEY_API_KEY`

### 5. APT Repository

**Hosting**: GitHub Pages на `accreation/aide-repo`

**Структура после публикации**:
```
aide-repo/ (gh-pages branch)
├── gpg.key                          # публичный ключ для импорта
├── pool/main/a/aide/
│   ├── aide_0.4.0_amd64.deb
│   └── aide_0.4.0_arm64.deb
└── dists/stable/
    ├── InRelease                    # Release + GPG подпись
    ├── Release                      # метаданные: дата, checksums
    └── main/
        ├── binary-amd64/
        │   ├── Packages
        │   └── Packages.gz
        └── binary-arm64/
            ├── Packages
            └── Packages.gz
```

**CI action**:
```bash
# 1. Клонируем aide-repo, переключаемся на gh-pages
# 2. Скачиваем .deb пакеты из текущего GitHub Release
# 3. Копируем в pool/main/a/aide/ (сохраняем старые версии)
# 4. Генерируем Packages:
#    apt-ftparchive packages pool/main > dists/stable/main/binary-amd64/Packages
#    apt-ftparchive packages pool/main > dists/stable/main/binary-arm64/Packages
#    gzip -k dists/stable/main/binary-*/Packages
# 5. Генерируем Release:
#    apt-ftparchive -c=apt-release.conf release dists/stable > dists/stable/Release
# 6. Подписываем: gpg --clearsign -o dists/stable/InRelease dists/stable/Release
# 7. Коммитим и пушим в gh-pages
```

`apt-release.conf` (в репо `aide`):
```
APT::FTPArchive::Release::Origin "Accreation";
APT::FTPArchive::Release::Label "Aide";
APT::FTPArchive::Release::Suite "stable";
APT::FTPArchive::Release::Codename "stable";
APT::FTPArchive::Release::Architectures "amd64 arm64";
APT::FTPArchive::Release::Components "main";
APT::FTPArchive::Release::Description "Aide — AI environment manager";
```

**Пользовательская установка**:
```bash
curl -fsSL https://accreation.github.io/aide-repo/gpg.key | sudo gpg --dearmor -o /etc/apt/trusted.gpg.d/aide.gpg
echo "deb https://accreation.github.io/aide-repo stable main" | sudo tee /etc/apt/sources.list.d/aide.list
sudo apt update && sudo apt install aide-cli
```

### 6. DNF Repository

**Hosting**: GitHub Pages на `accreation/aide-repo` (тот же репо, другая директория)

**Структура**:
```
aide-repo/ (gh-pages branch)
├── gpg.key
├── aide.repo                        # статичный .repo файл для dnf
├── aide/
│   ├── x86_64/
│   │   ├── aide-0.4.0-1.amd64.rpm
│   │   └── aide-0.3.0-1.amd64.rpm  # старые версии сохраняем
│   └── aarch64/
│       ├── aide-0.4.0-1.arm64.rpm
│       └── aide-0.3.0-1.arm64.rpm
└── repodata/                        # генерируется createrepo_c
    ├── repomd.xml                   # подписывается GPG
    └── ...
```

`aide.repo` (статический файл):
```ini
[aide]
name=Aide — AI environment manager
baseurl=https://accreation.github.io/aide-repo/aide
enabled=1
gpgcheck=1
gpgkey=https://accreation.github.io/aide-repo/gpg.key
```

**CI action**:
```bash
# 1. Клонируем aide-repo, переключаемся на gh-pages
# 2. Скачиваем .rpm пакеты из текущего GitHub Release
# 3. Копируем в aide/x86_64/ и aide/aarch64/
# 4. createrepo_c aide/            # генерирует repodata/
# 5. gpg --detach-sign --armor repodata/repomd.xml  # подпись
# 6. Коммитим и пушим в gh-pages
```

**Пользовательская установка**:
```bash
sudo dnf config-manager --add-repo https://accreation.github.io/aide-repo/aide.repo
sudo dnf install aide-cli
```

## CI Pipeline Changes (`release.yml`)

Добавляется новый job `publish` после `release`, который параллельно запускает все публикации:

```yaml
publish:
  name: Publish to package managers
  runs-on: ubuntu-latest
  needs: release
  steps:
    - uses: actions/checkout@v4
    
    - name: Publish to Homebrew Tap
      run: ./packaging/publish-homebrew.sh
    
    - name: Publish to Scoop Bucket
      run: ./packaging/publish-scoop.sh
    
    - name: Publish to Winget
      env:
        WINGET_GITHUB_TOKEN: ${{ secrets.WINGET_GITHUB_TOKEN }}
      run: ./packaging/publish-winget.sh
    
    - name: Publish to Chocolatey
      env:
        CHOCOLATEY_API_KEY: ${{ secrets.CHOCOLATEY_API_KEY }}
      run: ./packaging/publish-choco.sh
    
    - name: Publish to APT/DNF repo
      env:
        GPG_PRIVATE_KEY: ${{ secrets.GPG_PRIVATE_KEY }}
        GPG_PASSPHRASE: ${{ secrets.GPG_PASSPHRASE }}
      run: ./packaging/publish-apt-dnf.sh
```

Либо каждый менеджер — отдельный job (параллельно) для скорости.

## Required Secrets

| Secret | Purpose | Where |
|--------|---------|-------|
| `GPG_PRIVATE_KEY` | Подпись APT/DNF репозиториев | repo secrets |
| `GPG_PASSPHRASE` | Пароль GPG-ключа (опционально) | repo secrets |
| `WINGET_GITHUB_TOKEN` | PAT для PR в microsoft/winget-pkgs | repo secrets |
| `CHOCOLATEY_API_KEY` | API-ключ Chocolatey | repo secrets |

## New Files in `aide` Repo

```
packaging/
├── render.sh                     # Общий скрипт рендеринга шаблонов (envsubst)
├── homebrew/
│   └── aide.rb.tmpl
├── scoop/
│   └── aide.json.tmpl
├── chocolatey/
│   ├── aide.nuspec.tmpl
│   └── tools/
│       └── chocolateyInstall.ps1.tmpl
├── apt/
│   └── apt-release.conf
├── dnf/
│   └── aide.repo
├── publish-homebrew.sh
├── publish-scoop.sh
├── publish-winget.sh
├── publish-choco.sh
└── publish-apt-dnf.sh
```

**Рендеринг шаблонов**: используется `envsubst` (доступен в ubuntu-latest) или `sed` для подстановки переменных `$VERSION`, `$SHA_*` в шаблоны. Скрипты публикации получают версию из `$GITHUB_REF` и SHA256 из `checksums.txt` релиза.

## Error Handling

| Manager | Failure mode | Behavior |
|---------|-------------|----------|
| Homebrew | Не удалось клонировать/запушить tap | Логгируем ошибку, не блокируем остальные |
| Scoop | Аналогично | Не блокируем |
| Winget | PR не создался (rate limit, token expired) | Логгируем, не блокируем |
| Chocolatey | API key invalid, moderation rejected | Логгируем, не блокируем |
| APT/DNF | GPG signing failed | Логгируем, не блокируем |

**Каждый менеджер фейлится независимо** — если winget не обновился, остальные всё равно публикуются.

## Out of Scope

- Версионирование пакетов (всегда latest release, без бета/rc-каналов)
- Автоматическое создание `aide-repo` через Terraform/Pulumi (создаём руками один раз)
- GitHub Enterprise / self-hosted runners
- Поддержка нескольких версий aide одновременно в apt/dnf (только latest + история)
- Scoop arm64 (пока нет бинарников для Windows ARM)
- Chocolatey moderation bypass (первый пакет проходит ручную модерацию)

## Testing Strategy

- **Скрипты публикации**: ручное тестирование на первом релизе после внедрения
- **Шаблоны**: unit-тесты на рендеринг (сравнение с эталонным выводом)
- **APT/DNF**: проверить `apt update && apt install aide-cli` и `dnf install aide-cli` после публикации
- **Homebrew**: `brew install accreation/tap/aide` после публикации
- **Scoop**: `scoop bucket add accreation https://github.com/accreation/scoop-bucket && scoop install aide`
