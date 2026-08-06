# Recipes Maintainer — Agent Role

## Overview

Создать новую роль для AI-агентов — `recipes-maintainer`. Роль отвечает за:

1. Добавление / удаление / обновление рецептов в `internal/installer/recipes.yaml`
2. Обслуживание человеко-читаемой страницы-каталога `docs/tools-catalog.md` с описаниями и ссылками на официальные ресурсы

Интерфейс: slash-команда `/rm {add|remove|update} pkg-name`.

## Architecture

```
/rm add pkg-name
      → copilot-instructions.md (dispatch)
        → .github/agent-roles/recipes-maintainer.md (role instructions)
          → исследует пакет (оф. сайт, PM-доступность)
          → редактирует recipes.yaml
          → обновляет docs/tools-catalog.md
          → валидирует (go vet, go test)
```

## Files

| File | Purpose |
|------|---------|
| `.github/agent-roles/recipes-maintainer.md` | Инструкция для агента: пошаговый алгоритм add/remove/update, формат рецептов, требования к исследованию пакета |
| `.github/copilot-instructions.md` | Добавить dispatch: `/rm` → читать и исполнять `agent-roles/recipes-maintainer.md` |
| `docs/tools-catalog.md` | Человеко-читаемая таблица всех тулзов из `recipes.yaml` с описаниями и ссылками |

## Slash Command: `/rm`

Три подкоманды:

| Command | Action |
|---------|--------|
| `/rm add pkg-name` | Исследовать пакет → добавить рецепт → обновить каталог |
| `/rm remove pkg-name` | Удалить рецепт → удалить из каталога |
| `/rm update pkg-name` | Обновить рецепт и/или описание в каталоге |

## Agent Role: `recipes-maintainer.md`

### Add Algorithm

1. Принять имя пакета от пользователя
2. Найти официальный сайт и GitHub-репозиторий пакета (web search)
3. Определить, через какие package managers доступен пакет на Windows/macOS/Linux:
   - Проверить winget (`winget search`), brew (`brew search`), apt, dnf, scoop, choco, npm, pip
   - Если доступен как GitHub Release — определить формат `github: "owner/repo pattern binary"`
4. Сформировать YAML-блок в соответствии с форматом `recipes.yaml`:
   ```yaml
   pkg-name:
     windows:
       - winget: PackageId
       - scoop: pkg
     macos:
       - brew: pkg
     linux:
       - apt: pkg
       - dnf: pkg
   ```
5. Вставить блок в `recipes.yaml` (алфавитный порядок)
6. Добавить строку в `docs/tools-catalog.md` с описанием и ссылкой
7. Запустить `go vet ./...` и `go test -race ./internal/installer/...`

### Remove Algorithm

1. Найти блок `pkg-name` в `recipes.yaml`
2. Удалить блок
3. Удалить строку из `docs/tools-catalog.md`
4. Запустить валидацию

### Update Algorithm

1. Найти блок `pkg-name` в `recipes.yaml`
2. Обновить по запросу пользователя (или переисследовать)
3. Обновить описание в `docs/tools-catalog.md` при необходимости
4. Запустить валидацию

### Validation

После любого изменения:
```bash
go vet ./...
go test -race ./internal/installer/...
```

## `docs/tools-catalog.md` Format

```markdown
# Aide — Supported Tools Catalog

> Auto-generated from `internal/installer/recipes.yaml`. Maintained by the recipes-maintainer agent (`/rm`).

| Tool | Description | Official | Windows | macOS | Linux |
|------|-------------|----------|---------|-------|-------|
| gh   | GitHub CLI — manage repos, PRs, issues from terminal | [github.com/cli/cli](https://github.com/cli/cli) | winget, scoop, choco | brew | apt, dnf |
| glab | GitLab CLI | [gitlab.com/gitlab-org/cli](https://gitlab.com/gitlab-org/cli) | winget | brew | apt, dnf |
| ...  | ... | ... | ... | ... | ... |
```

- **Description**: краткое (5-10 слов), что делает инструмент
- **Official**: ссылка на GitHub-репо или оф. сайт
- **Platform columns**: перечисление доступных package managers через запятую

## `copilot-instructions.md` Changes

Добавить секцию:

```markdown
## Agent Roles

- **`/rm {add|remove|update} <pkg>`** — Recipe maintainer. When invoked, read and follow `.github/agent-roles/recipes-maintainer.md`.
```

## Out of Scope

- Автоматический PR/commit — агент редактирует файлы, пользователь сам коммитит
- Валидация рецептов на всех платформах (только go vet + тесты)
- Миграция старых рецептов (только add/remove/update текущих)
- Локализация описаний (только английский)
