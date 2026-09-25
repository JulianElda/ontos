package ontos

import "strings"

// Rules are the standing instructions `context` prints at the end of every
// session start. One copy lives here rather than in each repo, so changing
// them is a rebuild, not an edit in every project.
//
// Go raw strings cannot hold a backtick, so the text writes code spans as
// {verb} and the replacer turns the braces into backticks.
var Rules = strings.NewReplacer("{", "`", "}", "`").Replace(`- An entry is wrong or out of date: {update} or {delete} it in the same session.
- You worked out something the next session would have to rediscover: {search} first, then
  {update} the matching entry or {add} a new one.
- A new kind of task that no entry covers: {add} one with a trigger worded as the situation.
- An entry grew past one question: split it into several {add}s plus a {delete}.
- Be precise: state the exact trigger and the exact rule. If you can't, don't write it.
- {confirmed} only for what you ran or read in code; otherwise {inferred} or {suspected}. Never
  write a suspected bug as fact.
- Never store secrets, tokens, credentials, or URLs with credentials in them.
- Plans and open work go to pragma; personal preferences go to auto memory.
- {ontos help} lists the verbs and their flags. Bodies go through stdin: {--body-file -}.
`)
