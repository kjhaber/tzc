# tzc
A simple Timezone Converter TUI utility.

I often struggle to mentally convert between timezones when looking at service log files and graphs which are usually in UTC, and comparing those to chat/email/messaging correspondence which is usually in local time.  There are other timezone conversion cases that I sometimes have trouble with too, like knowing local time for coworkers in India or Europe (I'm in US West Coast currently).

I've limped by for years with various timezone converter websites or just Googling "local time in <whatever city>" but I've been meaning to come up with a nicer local-only utility that works with my terminal-heavy workflow for ages.  Now that it's 2026 and vibe-coding tools are all the rage, this little terminal app is now a reality.

## Installation

```bash
brew install kjhaber/tap/tzc
```

Or build from source (requires **Go 1.26** or later):

```bash
git clone https://github.com/kjhaber/tzc.git
cd tzc
go build -o tzc .
```

The Makefile can also produce `build/tzc`: run `make build` (or `make` for format check, lint, tests, and build).

## Usage

Run the TUI:

```bash
tzc
```

**UTC** and **Local** are always shown. Type a time in the focused row and press **Enter** to interpret it in that row’s timezone and fill the other rows with the same instant in their zones. **Esc** clears all fields.

**Navigation:** **Tab** / **Shift+Tab** or **↑** / **↓** move focus between rows.

**Extra timezones:** **+** opens “add timezone”; enter an [IANA](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) name (for example `America/New_York` or `Asia/Kolkata`, in any casing/spacing) or a common region name or abbreviation (like `mountain`, `edt`, `ist`, `jst`, `aest`, `india`, `sydney`) and press **Enter** to save. New zones are remembered in your user config directory under `tzc/zones` (for example `~/.config/tzc/zones` on Linux, or the macOS equivalent under **Application Support** when using the default config root). To remove a user-added row, focus it and press **Ctrl+D**, or clear its input and press **-**.

**Input examples** (the focused row’s placeholder shows similar hints): time-only values like `9pm` or `21:15`, calendar dates with times, RFC3339 / ISO-style strings, Unix epoch seconds, common log-style timestamps, and written-out dates like `Thu, 10 Sep 2026 19:21:53 GMT` (e.g. pasted from a browser dialog) or `Sep 10, 2026 7:21pm IST`. If you omit a date, today is assumed in that row’s zone; if you omit a timezone, the parser uses the focused row’s zone. Recognized timezone abbreviations (`IST`, `JST`, `AEST`, `CET`, …) get their real offset even if the row you typed them into doesn't natively know that abbreviation. Abbreviations that name more than one real zone (like `IST` or `CST`) resolve to the most common interpretation.

**CLI:**

```bash
tzc --version   # also -v, -version
```

Press **Ctrl+C** to quit.

## License
MIT

