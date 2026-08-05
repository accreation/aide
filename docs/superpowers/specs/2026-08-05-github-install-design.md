# GitHub Releases Install — Design Spec

## Overview

Добавить в `recipes.yaml` поддержку установки бинарников напрямую из GitHub Releases. Это решает проблему с инструментами, которых нет в пакетных менеджерах (winget/scoop/choco/brew/apt/dnf) — например, RTK для Windows.

Новый тип пакетного менеджера `github` скачивает ассет из последнего релиза GitHub-репозитория, распаковывает (если архив) и помещает бинарник в `~/.local/bin`.

## Architecture

Добавляем новую ветку в `Installer.Install()`:

```
ResolvePM(tool) → github?
  → fetch latest release from GitHub API
  → resolve template variables in asset name
  → match asset by pattern
  → download asset
  → extract if archive (.zip, .tar.gz)
  → place binary in ~/.local/bin
  → ensure dir in PATH
```

### Новые файлы

| File | Purpose |
|------|---------|
| `internal/installer/github.go` | Логика установки из GitHub Releases |
| `internal/installer/github_test.go` | Тесты для github-установщика |
| `internal/installer/path.go` | Управление `~/.local/bin` в PATH |
| `internal/installer/path_test.go` | Тесты |

### Изменяемые файлы

| File | Change |
|------|--------|
| `internal/installer/recipes.go` | Добавить `ArchMap` в `Recipe`, поддержку template-переменных в `PMEntry` |
| `internal/installer/recipes.yaml` | Добавить рецепт `rtk` с `github`-записью для Windows |
| `internal/installer/recipes_test.go` | Тесты на парсинг `arch_map` и template-переменных |
| `internal/installer/installer.go` | Добавить ветку `pm == "github"` в `Install()` |

## Recipe Format

```yaml
rtk:
  arch_map:
    amd64: x86_64
    arm64: aarch64
  windows:
    - github: "rtk-ai/rtk rtk-${ARCH}-pc-windows-msvc.zip rtk.exe"
  macos:
    - brew: rtk
  linux:
    - curl: "https://raw.githubusercontent.com/rtk-ai/rtk/refs/heads/master/install.sh | sh"
```

| Field | Required | Description |
|-------|----------|-------------|
| `arch_map` | No | Маппинг GOARCH → целевая архитектура. Используется для `${ARCH}`. Если не указан, `${ARCH}` = `${GOARCH}`. |
| `github` entry | — | Формат: `"owner/repo asset-шаблон бинарник"` — три поля через пробел. `бинарник` — имя файла внутри архива (для `.zip`/`.tar.gz`) и одновременно имя, под которым он сохраняется в `~/.local/bin`. |

### Template Variables

Разрешаются в рантайме при обработке значений `github` и `curl`:

| Variable | Source | Example |
|----------|--------|---------|
| `${GOARCH}` | `runtime.GOARCH` | `amd64`, `arm64`, `386` |
| `${GOOS}` | `runtime.GOOS` | `windows`, `darwin`, `linux` |
| `${OS}` | `runtime.GOOS` (alias) | `windows`, `darwin`, `linux` |
| `${ARCH}` | `arch_map[GOARCH]`, fallback to `GOARCH` | `x86_64`, `aarch64` |

## GitHub Install Flow

```
1. GET https://api.github.com/repos/{owner}/{repo}/releases/latest
2. Parse JSON → .assets[].name, .assets[].browser_download_url
3. Find asset where name matches pattern (after template substitution)
4. Download to temp dir
5. If .zip → unzip; if .tar.gz → untar+gzip; otherwise → plain binary
6. Copy binary to ~/.local/bin/{binary-name} (chmod +x on Unix)
7. Ensure ~/.local/bin in PATH (once, idempotent)
```

### Extraction

- **`.zip`**: `archive/zip` stdlib
- **`.tar.gz`**: `archive/tar` + `compress/gzip` stdlib
- **Plain binary**: rename directly

Из архива извлекается файл, чьё имя совпадает с `имя-бинарника` (или файл с таким именем без расширения).

### PATH Management

При первом успешном `github`-инсталле:

- **Windows**: записать `%USERPROFILE%\.local\bin` в `HKCU\Environment\Path` через реестр, broadcast `WM_SETTINGCHANGE`
- **Linux/macOS**: дописать `export PATH="$HOME/.local/bin:$PATH"` в `~/.bashrc` и `~/.zshrc` (только если файл существует и строки ещё нет)

В обеих ОС — операция идемпотентная: если путь уже есть, не дублируем.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| GitHub API недоступен | Ошибка с пояснением, перейти к следующему PM-записи |
| Релиз не найден | Ошибка "no releases for {repo}" |
| Ассет не найден по шаблону | Ошибка "no asset matching '{pattern}' in latest release" |
| Архив не содержит бинарник | Ошибка "binary '{name}' not found in archive" |
| Не удалось записать PATH | Вывести инструкцию вручную, не фейлить установку |

## API / HTTP

- URL: `https://api.github.com/repos/{owner}/{repo}/releases/latest`
- Без аутентификации (публичные репозитории)
- Rate limit: 60 req/hour без токена. **При private-репозиториях** в будущем — поддержка `GITHUB_TOKEN`.
- Timeout: 30 секунд на запрос + загрузку

## Testing

- **Unit**: template resolution, arch_map parsing, asset name matching
- **Unit**: zip/tar.gz extraction (с фиктивными архивами в памяти)
- **Unit**: PATH-функции (Windows registry mock, bashrc/zshrc string manipulation)
- **Unit**: парсинг ответа GitHub API (фикстурные JSON)

## Out of Scope

- GitHub Enterprise (кастомный API URL)
- Private-репозитории с аутентификацией
- Проверка checksum/signature скачанных ассетов
- Версионирование для github-install (всегда latest release)
- Поддержка `.tar.xz`, `.7z` и других форматов архивов
