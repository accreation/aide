# Aide — AI Environment Manager

## Overview

**Aide** — CLI-инструмент на Go, который управляет AI-окружением разработчика через декларативный конфиг `aide.yaml`. Проверяет наличие провайдера (Claude, Copilot, Codex, и т.д.) и всех необходимых инструментов, устанавливает недостающее, и запускает AI-провайдера.

Аналог `package.json` для AI-окружения: один конфиг описывает, что должно быть установлено, а Aide приводит систему в соответствие.

## Architecture

```
aide.yaml  →  [Parser]  →  [Checker]  →  [Installer]  →  [Launcher]
                  ↑              ↑
             recipes.yaml   recipes.yaml
```

### Components

| Component  | Responsibility |
|------------|---------------|
| **Parser** | Читает и валидирует `aide.yaml`. Возвращает структуру с provider + tools. |
| **Checker** | Проверяет наличие провайдера и тулзов в PATH, сравнивает версии через semver-constraints. |
| **Installer** | По `recipes.yaml` определяет способ установки под текущую ОС и доступные пакетные менеджеры. |
| **Launcher** | Запускает провайдера командой оболочки. |

`recipes.yaml` — встроен в бинарник (embed), но может быть переопределён внешним файлом для обновлений без перекомпиляции.

## Configuration Format

### `aide.yaml`

```yaml
provider: claude

tools:
  - name: gh
    version: ">=2.0.0"
  - name: az
  - name: glab
```

| Field      | Required | Description |
|------------|----------|-------------|
| `provider` | Yes      | Имя AI-провайдера: `claude`, `copilot`, `codex`, `opencode` |
| `tools`    | Yes      | Список необходимых CLI-инструментов |
| `tools[].name` | Yes | Имя исполняемого файла |
| `tools[].version` | No | Semver-constraint (`>=`, `^`, `~`, `=`, диапазоны). Если не указана — любая версия. |

### `recipes.yaml` (встроенный)

```yaml
gh:
  windows:
    - winget: GitHub.cli
    - scoop: gh
    - choco: gh
  macos:
    - brew: gh
  linux:
    - apt: gh
    - dnf: gh

glab:
  windows:
    - winget: GitLab.GitLab
  macos:
    - brew: glab
  linux:
    - apt: glab
    - dnf: glab

az:
  windows:
    - winget: Microsoft.AzureCLI
  macos:
    - brew: azure-cli
  linux:
    - curl: "https://aka.ms/InstallAzureCLIDeb | bash"
```

Приоритет менеджеров: первый доступный в системе. Проверяется наличие `winget`, `scoop`, `choco`, `brew`, `apt`, `dnf` — и используется первый найденный.

## CLI Interface

```bash
aide                    # check: проверка окружения
aide check              # явный check
aide install            # установка недостающего
aide i                  # алиас для install
aide init               # создаёт aide.yaml в текущей директории
```

### Flow

1. Найти `aide.yaml` — текущая директория, затем вверх по дереву (как `.git`)
2. Распарсить и провалидировать
3. Проверить provider (установлен ли бинарник)
4. Проверить каждый tool: есть ли в PATH, соответствует ли версия
5. Вывести результат — зелёные ✓ и красные ✗
6. В режиме `install` — для каждого ✗ найти рецепт в `recipes.yaml` и установить
7. Если всё ✓ — запустить провайдера

### Output Example

```
✓ provider: claude
✓ gh v2.65.0 (>=2.0.0)
✗ glab — not found
✗ az — version 2.50.0, required >=2.60.0
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0    | Всё ok, провайдер запущен |
| 1    | Что-то не установлено / ошибка |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| `aide.yaml` не найден | Сообщение + предложение `aide init` |
| Невалидный YAML | Указать строку и проблему |
| `version` не парсится как semver | Предупредить, предложить корректный формат |
| Tool/provider не установлен | Check: рапорт. Install: попытка установки |
| Нет рецепта для tool | Сообщить "no known recipe for [tool] on [OS]", exit 1 |
| Установка не удалась | Показать stderr, продолжить со следующим, exit 1 в конце |
| Провайдер не запустился | Показать ошибку, exit 1 |

## Testing

- **Unit tests**: parser, semver comparison, recipe resolver (с моком OS/менеджеров)
- **Integration tests**: Docker-контейнеры под Linux, полный цикл `check` + `install`
- **Smoke test**: `go build && ./aide check` на тестовом `aide.yaml`

## Tech Stack

- **Language**: Go 1.22+
- **Dependencies**: `go-yaml/yaml` (YAML), `Masterminds/semver` (semver), `spf13/cobra` (CLI)
- **Build**: `go build`, cross-compilation для Windows/macOS/Linux

## Out of Scope (v1)

- Установка рантаймов (go, node, python)
- Переменные окружения в конфиге
- Кастомные скрипты установки в `aide.yaml`
- Конфигурация/интеграция с провайдером (только запуск)
