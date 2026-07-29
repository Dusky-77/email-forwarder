# email-forwarder

watches one or more gmail accounts and forwards matching emails to discord as embeds, with pings. built in go.

useful if you get important mail scattered across multiple gmail accounts and want a single discord channel to actually notice it in.

## what it does

- polls multiple gmail accounts on a timer (gmail api, oauth2, no imap/app passwords needed)
- matches incoming mail against rules you write: sender email, sender domain, sender name, or case sensitive keywords in subject/body
- forwards a match as a discord embed via webhook, including a real `<t:unix:F>` timestamp so discord shows it in everyone's own local time
- pings whoever you configure, per rule, mixing specific users and specific roles, or falling back to a shared default list
- tracks what it's already forwarded so restarting the process never double-sends

## how matching works

each rule can set any combination of:

- `SenderEmails` — exact match against the sender's email address
- `SenderDomains` — matches anything `@thatdomain.com`
- `SenderNameContains` — partial match against the sender's display name
- `Keywords` — case sensitive, checked against subject and body separately, the discord embed tells you which one hit

fields you set on a rule are ANDed together (all of them must pass). within a field, it's OR (any one entry in the list is enough). rules are checked top to bottom and the first one that matches wins, so put more specific rules above general catch-alls.

leave a rule with zero fields set and it just won't match anything, so don't do that.

## setup

### 1. get gmail api credentials

for each gmail account you want to watch:

1. go to the [google cloud console](https://console.cloud.google.com/)
2. create a project (or use an existing one)
3. enable the **gmail api** for that project
4. go to "credentials" → "create credentials" → "oauth client id"
5. application type: **desktop app**
6. download the json, this is your `credentials.json` for that account, paste it into `config/` as `config/whatever the name is.json`.
7. then change the path `CredentialsFile` in `config.go` to point to your new credentials file.

do this once per google cloud project. you can reuse one project for multiple gmail accounts, you just need to go through the oauth consent screen separately for each gmail account.

### 2. clone and configure

```bash
git clone https://github.com/Dusky-77/email-forwarder.git
cd email-forwarder
mkdir -p config
```

drop each account's downloaded credentials json into `config/`, e.g. `config/personal_credentials.json`.

now open `config.go` and edit it directly. this is the only file you need to touch:

- `Accounts` — one entry per gmail inbox, pointing at its credentials file
- `Rules` — your forwarding rules, see above
- `DefaultPing` — fallback ping list for rules that don't set their own
- `PollIntervalMinutes` — how often to check, 2-5 min is a safe range
- `DiscordTimestampStyle` — `F` for full date/time, `R` for relative, etc (see discord's timestamp formatting docs)
- `BodySnippetLength` — how much of the email body shows in the embed

### 3. get your discord webhook urls

in discord: channel settings → integrations → webhooks → new webhook → copy url. make one per channel you want to forward into, then paste them into the relevant rules' `WebhookURL` field.

### 4. build and run

```bash
go mod tidy
go build -o email-forwarder .
./email-forwarder
```

first run, for each account, it'll print a url in the terminal. open it, log into that gmail account, approve access, and paste the code back into the terminal. after that it caches a token file and won't ask again unless you delete it.

## running it long term (pterodactyl / vps)

this is a normal long running go process, no special handling needed. treat it like any other bot:

- build the binary, upload it, run it
- or just run `go run .` inside a persistent session/service if you'd rather not deal with the binary

if you want crash alerts, wrap the process in whatever you're already using for your other bots (pm2, systemd, a supervisor script that pings a webhook on exit, etc). this project doesn't include that itself since it depends on your setup.

## project structure

```
email-forwarder/
├── main.go              # entrypoint, one poll loop per account
├── config.go            # everything you're meant to edit lives here
├── gmail/
│   ├── auth.go          # oauth flow + token caching
│   └── watcher.go       # fetches new mail since last check
├── matcher/
│   └── rules.go         # runs a message against your rule list
├── discord/
│   └── forwarder.go     # builds the embed, resolves pings, sends it
└── store/
    └── seen.go          # tracks last processed gmail historyId per account
```

## notes / limitations

- gmail only, no other providers, since this uses the gmail api directly and not imap
- rule names should be unique, if two rules share a name the wrong one can get looked up internally
- if an account's history id goes stale (gmail expired it, usually only happens after being offline a long time), the fetch will error out for that cycle instead of silently skipping mail, check your logs if an account goes quiet
- this reads mail, it doesn't delete, label, or modify anything in your inbox

## license

MIT, see [LICENSE](LICENSE). do whatever you want with it.
