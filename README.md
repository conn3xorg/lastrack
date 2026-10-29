# Last Rack Standing

Ten racks, three edge nodes, and a sky full of things that want to page you.
`lastrack` is a small terminal arcade game from the Conn3x crew. You won't find
it in the docs. You found it anyway.

From a checkout, with Go 1.24 or later:

```sh
make build && ./lastrack
```

Tagged releases also carry static Linux binaries for amd64 and arm64, with a
`SHA256SUMS` to check them against. It runs over plain SSH, inside screen or
tmux, and needs an 80x24 terminal.

## The shift

Threats fall toward your racks. Put the cursor in their path and fire; the
nearest edge node sends an interceptor, and where it lands it opens a scrub
blast. Anything caught in a blast is scrubbed and blows a smaller blast of its
own, so one good shot can take out a whole formation. Each link down that chain
multiplies the points.

| Threat | Points | What it does |
| --- | --- | --- |
| `◆` rogue packet | 10 | Comes straight down. |
| `▼` bad deploy | 40 | Fans out into three fragments (15 each) partway down. |
| `!` 3 a.m. page | 75 | Slow, but it keeps re-aiming at the nearest live rack, and it takes the rack down in one hit. |

A rack takes two hits: the first leaves it degraded (yellow), the second turns
it dark. Lose all ten and the shift is over. Nodes that take a hit reboot for
ten seconds.

Firing costs bandwidth, from one shared pool of ten. It comes back on its own,
a unit every two-thirds of a second, so spraying shots will leave you empty at
the worst moment. Every rack still standing at the end of a wave pays an uptime
bonus, and every 5,000 points a circuit rebuilds your most damaged rack.

## Controls

| | |
| --- | --- |
| Move the cursor | arrow keys, `wasd`, `hjkl`, or the mouse |
| Fire from the nearest node | `space` or click |
| Fire from a given node | `1`, `2`, `3` |
| Pause | `p` |
| Quit | `q` or Ctrl-C |

## Flags

- `-ascii` swaps every glyph for plain ASCII, for fonts that don't have them.
- `-seed N` fixes the random seed. Same seed, same inputs, same game.
- `-version` prints the version.

Colours stick to the basic 16, so an old terminal looks right too. Your high
score lives in `$XDG_STATE_HOME/lastrack/highscore`, or
`~/.local/state/lastrack/highscore` without it. If that can't be written, the
game carries on without it.

## Building

```sh
make test      # gofmt, go vet, go test
make build     # ./lastrack
make release   # static linux/amd64 and linux/arm64 in dist/, with SHA256SUMS
```

The rules live in `game/` and know nothing about terminals, which is what keeps
them testable. The only dependency is [tcell](https://github.com/gdamore/tcell).

## Thanks

To the people who built the cabinets and the people who built the terminals:
you made a grid of characters feel like somewhere to be.

## License

[MPL-2.0](LICENSE).
