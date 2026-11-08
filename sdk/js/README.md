# @rendau/ruto-auth

Браузерный SDK для входа через шлюз ruto. Шлюз держит сессию в httpOnly-куках
на своём хосте, токены в браузер не попадают. Фронту остаётся три вещи, и все
они здесь:

- отправлять запросы с куками и после `401` один раз обновить сессию
  (`POST …/refresh`) и повторить запрос;
- узнать, кто вошёл (`GET …/me` — claims access-токена как есть);
- увести браузер на вход и на выход (`…/login`, `…/logout`) и прочитать код
  ошибки, с которым шлюз вернул браузер после неудачного входа (`?auth_error=`).

SDK не привязан к HTTP-клиенту: `run()` оборачивает запрос любой библиотеки,
`fetch` — готовая обёртка над fetch, а `./axios`, `./ofetch` и `./ky` встраиваются
в хуки этих библиотек. Зависимостей нет, размер — около 10 КБ без сжатия.

## Установка

Пакет живёт в репозитории ruto (`sdk/js`), в npm не публикуется. Подключение
из git, подпапкой:

```bash
pnpm add "github:rendau/ruto#path:/sdk/js"
```

(для npm: `npm i "github:rendau/ruto#path:/sdk/js"`). Собранного `dist` в git
нет: при установке из git pnpm и npm сами запускают `prepare`, то есть сборку.
Для страницы без сборщика — файл `dist/ruto-auth.iife.js` после `pnpm build`,
всё лежит в глобальном объекте `RutoAuth`.

## Что нужно на стороне шлюза

- В Root включён провайдер входа (`jwt[].name`, `issuer`, `client`), у
  приложения метод `jwt` с этим провайдером.
- Origin фронта (`https://desk.example.com`) перечислен в `cors.allow_origins`
  и `allow_credentials = true`. Иначе браузер не отдаст скрипту ответы `me` и
  `refresh`, а `login` не примет такой `redirect`.
- Адрес возврата (`redirect` у `login`/`logout`) — либо путь на шлюзе, либо
  абсолютный адрес на origin из CORS. По умолчанию SDK подставляет текущую
  страницу (вход) и корень сайта (выход).

## Быстрый старт

```ts
import { createRutoAuth, hasRole, usernameOf } from '@rendau/ruto-auth'

const auth = createRutoAuth({
  gateway: 'https://api.mdev.kz',
  provider: 'corp',
  // app: '/orders'  — только если у приложения свой клиент (App.jwt)
  onLoginRequired: () => auth.login(), // сессия закончилась — на вход
})

const me = await auth.me() // claims или null, если не вошёл
if (!me) auth.login()
console.log(usernameOf(me), hasRole(me, 'shop:manager'))

const r = await auth.fetch('https://api.mdev.kz/shop/orders') // куки + повтор после 401
```

### Единица входа

Провайдер даёт общий вход на все приложения: маршруты
`<gateway>/_auth/<provider>/…`, кука на весь хост. Если у приложения свой
клиент (`App.jwt`), у него отдельная сессия: передайте `app: '/<префикс>'`, и
SDK сам соберёт адреса `<gateway>/_auth/<provider>/<префикс>/…` и
`<gateway>/<префикс>/_auth/<provider>/me`.

## API

| Метод | Что делает |
|---|---|
| `me()` | `Promise<Claims \| null>`. Claims access-токена; после `401` один раз пробует `refresh`. `null` — сессии нет (без вызова `onLoginRequired`). Ошибка шлюза или сети — исключение. |
| `run(request, { statusOf? })` | Выполняет `request()`; если результат или брошенная ошибка — `401`, обновляет сессию и повторяет запрос один раз. Статус читается из `Response`, ответа и ошибки axios, `FetchError` ofetch, `HTTPError` ky, `XMLHttpRequest`; для другого — свой `statusOf`. |
| `fetch(input, init)` | fetch с `credentials: 'include'` внутри `run`. |
| `refresh()` | `Promise<'ok' \| 'expired' \| 'failed'>`. Одновременные вызовы делят один запрос; между вкладками — Web Lock, если браузер его даёт. |
| `recover()` | Для адаптеров: `refresh()` плюс вызов `onLoginRequired` (один раз на обновление, если сессия закончилась); `true` — повторяйте запрос. |
| `login(redirect?, { prompt? })`, `logout(redirect?)` | Уводят браузер на вход/выход; `loginUrl()`, `logoutUrl()` — только адреса. `prompt` — стандартный параметр OpenID Connect, шлюз передаёт его провайдеру как есть: `'login'` — спросить пароль заново даже при живой SSO-сессии (после отказа в доступе входит другой человек), `'select_account'`, `'consent'`, `'none'`; несколько — списком. |
| `loginError(url?)` | Код неудачного входа из адреса страницы (`?auth_error=…`), иначе `null`. Шлюз возвращает браузер на адрес из `login` с этим параметром: код провайдера как есть (`access_denied`, `login_required`), `invalid_grant`, `temporarily_unavailable`, свои `state_mismatch` и `invalid_token`. |
| `rolesOf(claims)` | `{ realm: string[], clients: Record<string, string[]> }` из `roles`/`role`, `realm_access.roles`, `resource_access.*.roles`. |
| `hasRole(claims, ...roles)` | Правило шлюза: `client:role` — роль только этого клиента, голая `role` — где угодно. |
| `usernameOf(claims)` | Первое из `preferred_username`, `upn`, `login`, `email`, `name`, `sub`. |

Результат `refresh`:

- `ok` — куки обновлены;
- `expired` — сессии нет (refresh-куки нет, провайдер отказал); пора на вход;
- `failed` — провайдер или сеть недоступны; сессия, возможно, жива, выходить незачем.

`onLoginRequired` вызывается только из `run`/`fetch`/адаптеров и только при
`expired` — один раз на одно обновление, сколько бы запросов ни получили `401`
одновременно. `me()` его не вызывает: «не вошёл» на публичной странице — не
повод для редиректа.

## Адаптеры

### axios

```ts
import axios from 'axios'
import { installAxios } from '@rendau/ruto-auth/axios'

const api = axios.create({ baseURL: 'https://api.mdev.kz' })
installAxios(auth, api) // withCredentials + повтор после 401; возвращает функцию снятия
```

### ofetch (`$fetch` в Nuxt)

```ts
import { ofetchOptions } from '@rendau/ruto-auth/ofetch'

const api = $fetch.create({ baseURL: 'https://api.mdev.kz', ...ofetchOptions(auth) })
```

ofetch не складывает хуки, а заменяет: `onResponseError` у отдельного вызова
перекроет адаптер. После успешного обновления повтор идёт собственным retry
ofetch (`retry: 1` только для `401`, без задержки).

### ky

```ts
import ky from 'ky'
import { kyOptions } from '@rendau/ruto-auth/ky'

// ky 2: `prefix`; ky 1: `prefixUrl`
const api: typeof ky = ky.create({ prefix: 'https://api.mdev.kz', ...kyOptions(auth, (req) => api(req)) })
```

Хук `afterResponse` повторяет запрос самим ky, поэтому ему передаётся инстанс
(в ky 2 хук получает объект состояния, в ky 1 — три аргумента; поддержаны оба).

### Любой другой клиент

```ts
const data = await auth.run(() => myClient.get('/orders'))
```

Если библиотека прячет статус, передайте `statusOf: (resultOrError) => number | undefined`.

### Страница без сборщика

```html
<script src="/ruto-auth.iife.js"></script>
<script>
  const auth = RutoAuth.createRutoAuth({ gateway: 'https://api.mdev.kz', provider: 'corp' })
  auth.me().then((me) => me ? show(RutoAuth.usernameOf(me)) : auth.login())
</script>
```

## Пример: Nuxt-админка с guard'ом

```ts
// utils/auth.ts
export const auth = createRutoAuth({
  gateway: apiUrl(),
  provider: 'corp',
  onLoginRequired: () => navigateTo({ path: '/login', query: { return_to: useRoute().fullPath } }),
})

// utils/api.ts
export const api = $fetch.create({ baseURL: apiUrl(), ...ofetchOptions(auth) })

// middleware/auth.global.ts
export default defineNuxtRouteMiddleware(async (to) => {
  if (to.path === '/login') return
  const me = await auth.me()
  if (!me) return navigateTo({ path: '/login', query: { return_to: to.fullPath } })
  if (!hasRole(me, 'desk:dispatcher', 'desk:worker', 'desk:admin')) return navigateTo('/no-access')
})

// pages/login.vue — показать, почему не вошли, и кнопка «Войти»; после отказа
// просим пароль заново, иначе SSO молча вернёт того же пользователя:
const error = loginError() // 'access_denied' | 'login_required' | … | null
auth.login(location.origin + String(route.query.return_to ?? '/'), { prompt: error ? 'login' : undefined })
```

Бэкенд при этом получает обычный `Authorization: Bearer …` от шлюза и свой
клиент сессий не держит: кто вошёл — `claims` из токена.

## Ограничения

- Повтор запроса заново вызывает переданную функцию: тело-поток
  (`ReadableStream`) второй раз не прочитается; обычные строки, JSON, `FormData`
  повторяются без проблем.
- SDK работает только в браузере с куками шлюза; для нативных приложений
  (Capacitor, React Native) нужен системный браузер для входа и общий cookie
  store — это отдельная задача.

## Разработка

```bash
pnpm install
pnpm test        # vitest: ядро, роли, адаптеры axios / ofetch / ky
pnpm build       # dist/: ESM + .d.ts, dist/ruto-auth.iife.js
```

Живая проверка — тестовый стенд репозитория (раздел «Тестовый стенд» в
`CLAUDE.md`): фронты `config/test-stand/fronts/*` собраны на этом SDK.
