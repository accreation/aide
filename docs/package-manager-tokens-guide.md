# Package Manager Publishing — Token & Key Setup Guide

Здесь пошагово описано, где и как получить все токены и ключи, нужные для автоматической публикации Aide в пакетные менеджеры.

---

## Уже сделано (Task 1)

Эти ключи уже сгенерированы и лежат в GitHub Secrets репозитория `accreation/aide`:

| Secret | Что это | Статус |
|--------|---------|--------|
| `GPG_PRIVATE_KEY` | Приватный GPG-ключ для подписи APT/DNF репозиториев | ✅ Готов |
| `GPG_PASSPHRASE` | Пароль GPG-ключа (пустой) | ✅ Готов |
| `HOMEBREW_TAP_DEPLOY_KEY` | SSH deploy key для `accreation/homebrew-tap` | ✅ Готов |
| `SCOOP_BUCKET_DEPLOY_KEY` | SSH deploy key для `accreation/scoop-bucket` | ✅ Готов |
| `AIDE_REPO_DEPLOY_KEY` | SSH deploy key для `accreation/aide-repo` | ✅ Готов |

---

## Нужно заменить (сейчас там плейсхолдеры)

### 1. `WINGET_GITHUB_TOKEN` — GitHub PAT для winget

**Зачем:** `wingetcreate` создаёт Pull Request в репозиторий `microsoft/winget-pkgs`. GitHub не разрешает использовать встроенный `GITHUB_TOKEN` Actions для PR в чужие репозитории, поэтому нужен отдельный Personal Access Token.

**Как получить:**

1. **Создай отдельный GitHub-аккаунт** (рекомендуется bot-аккаунт, например `accreation-bot`). Можно использовать и свой личный, но bot-аккаунт безопаснее — если токен скомпрометирован, пострадает только он.

2. Войди под этим аккаунтом → **Settings → Developer settings → Personal access tokens → Tokens (classic)**. *(Не Fine-grained — в них права на PR не даются для публичных репозиториев.)*

3. Нажми **Generate new token → Generate new token (classic)**.

4. Заполни:
   - **Note**: `aide-winget-publisher`
   - **Expiration**: `90 days` (или `No expiration`)
   - **Scopes**: отметь **только** `public_repo` — это даёт доступ на создание PR в любые публичные репозитории, включая `microsoft/winget-pkgs`

5. Нажми **Generate token**, скопируй значение (начинается с `ghp_`).

6. Установи в секреты:
   ```bash
   gh secret set WINGET_GITHUB_TOKEN --repo accreation/aide --body "ghp_XXXXXXXXXXXXXXXXXXXX"
   ```

> ⚠️ **Важно:** У classic-токенов есть срок действия. Поставь напоминание обновить за неделю до истечения, иначе winget-публикация начнёт падать.

---

### 2. `CHOCOLATEY_API_KEY` — API-ключ Chocolatey

**Зачем:** `choco push` отправляет пакет в Chocolatey Community Repository. Для этого нужен API-ключ.

**Как получить:**

1. Зарегистрируйся на [community.chocolatey.org](https://community.chocolatey.org/account/Register).

2. Подтверди email.

3. Зайди в **Account → API Keys**.

4. API-ключ показан на странице — строка вида `choco_XXXXXXXXXXXXXXXXXXXX`.

5. Если ключа нет, нажми **Generate API Key**.

6. Скопируй ключ и установи в секреты:
   ```bash
   gh secret set CHOCOLATEY_API_KEY --repo accreation/aide --body "choco_XXXXXXXXXXXXXXXXXXXX"
   ```

> ⚠️ Первый пакет проходит ручную модерацию (1-3 дня). После одобрения все последующие версии обновляются автоматически без модерации.

---

## Дополнительно: ручные действия

### Первая публикация в Winget

После замены `WINGET_GITHUB_TOKEN` нужно один раз вручную создать пакет в `microsoft/winget-pkgs`:

```bash
# Установи wingetcreate
dotnet tool install --global Microsoft.WingetCreate

# Создай первый манифест
wingetcreate new Accreation.Aide \
  --version 0.4.0 \
  --urls "https://github.com/accreation/aide/releases/download/v0.4.0/aide-windows-amd64.exe.zip" \
  --publisher Accreation \
  --name Aide \
  --description "The package.json for your AI development environment"

# Отправь PR
wingetcreate submit --token "github_pat_..."
```

После того как PR примут (1-3 дня, модерация Microsoft), все следующие релизы будут обновляться автоматически через CI (`wingetcreate update`).

### Включение GitHub Pages для `aide-repo`

Если ещё не включено:

1. Зайди в `github.com/accreation/aide-repo` → **Settings → Pages**
2. **Source**: `Deploy from a branch`
3. **Branch**: `gh-pages`, `/(root)`
4. Нажми **Save**

Проверь: открой `https://accreation.github.io/aide-repo/gpg.key` — должен вернуть публичный GPG-ключ.

---

## Проверка всех секретов

```bash
gh secret list --repo accreation/aide
```

Ожидаемый вывод (7 секретов):
```
AIDE_REPO_DEPLOY_KEY
CHOCOLATEY_API_KEY
GPG_PASSPHRASE
GPG_PRIVATE_KEY
HOMEBREW_TAP_DEPLOY_KEY
SCOOP_BUCKET_DEPLOY_KEY
WINGET_GITHUB_TOKEN
```

---

## Проверка первого релиза

После того как все токены на месте, запушь тег и смотри логи:

```bash
git tag -a v0.5.0 -m "v0.5.0: test package manager publishing"
git push origin v0.5.0

# Смотри вкладку Actions в репозитории
gh run watch
```

Ожидаемое поведение:
- Homebrew: формула обновилась в `accreation/homebrew-tap`
- Scoop: манифест обновился в `accreation/scoop-bucket`
- Winget: PR создан в `microsoft/winget-pkgs` (после первого ручного одобрения)
- Chocolatey: пакет запушен (после первой ручной модерации)
- APT/DNF: пакеты добавлены в `accreation.github.io/aide-repo`
