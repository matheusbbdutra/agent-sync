# Rust

> Language reference for Rust code in this environment. State: **active** (`skills/backend/reference/rust.md`).
> Factual basis: the two Rust crates of the environment — `~/Projects/phpide-rs` and `~/Projects/pi-deck/src-tauri`. Local toolchain: `rustc 1.99.1`, `edition = "2021"` in both.

## Verified context

| crate | direct deps | async runtime |
|---|---|---|
| `phpide-rs` | `eframe`/`egui` 0.37, `tree-sitter` 0.27, `tree-sitter-php` 0.25 (`Cargo.toml:9-12`) | none |
| `pi-deck` | `tauri` 2, `serde_json` 1, `base64` 0.21 (`Cargo.toml:11-13`) | none direct; `tokio 1.54.1` only transitive via `tauri 2.12.5` (`cargo tree -i tokio`) |

Consequence: **there is no `tokio`, `clap`, `thiserror` nor `anyhow` as a direct dependency**. This guide describes what exists and what to adopt when the problem appears; it does not prescribe a new stack.

## 1. When to use Rust here (and when not)

Use when ≥2 criteria below are true and verifiable:
1. **Delivery as a native binary without external runtime.** Both are desktop apps; `pi-deck` even turns off the console on Windows (`src-tauri/src/main.rs:1`).
2. **Interop with a C/C++ library.** `tree-sitter` is consumed directly (`highlight.rs:9`), without its own binding layer.
3. **Predictable memory/latency, without GC** — ownership frees at the end of the scope (Rust Book ch. 4.1).
4. **Long process with shared state** — `pi-deck` keeps `Mutex<HashMap<String, ChildHandle>>` with live subprocesses (`rpc.rs:24`).

Do not use:
- **No measured bottleneck.** Without a profiler or a numeric latency/memory target, the rewrite is cost without measurable gain.
- **Graph with cycles and pointer to "parent".** Requires `Rc<RefCell<_>>` + `Weak`, and `Rc` is `!Send` (std docs), which blocks use across threads. Without an arena (`Vec` + index) the borrow checker becomes daily friction.
- **CRUD/prototype with a short deadline and TS/Go stack already installed.** The time to first deploy is longer; memory safety does not pay for the difference.
- **Team without experience and closed schedule.** The cost of learning borrowing is real and lands in the first weeks.

## 2. Ownership/borrowing

Three rules (Rust Book ch. 4.1, "Ownership Rules"): every value has an owner; there is one owner at a time; on leaving the scope the value is freed.

Pattern for "the borrow checker does not allow it" — in this order, stop at the first one that solves it:

1. **Shorten the loan.** The borrow ends at the last use, not at the end of the block; moving the mutation to afterwards solves most E0502 (verified with `rustc 1.99.1`).
2. **Borrow the field, not the whole `self`.** `fn get(&self) -> &Vec<u8>` ties the reference to the entire `self`; returning `&[u8]` of the field or splitting into smaller structs frees the mutation.
3. **Trade ownership for borrowing in the signature.** In E0382 rustc itself emits `note: consider changing this parameter type ... to borrow`. `&str`/`&[T]` in the parameter; `String`/`Vec<T>` only with real ownership.
4. **Iterate differently.** `for x in &v { v.push(..) }` is E0502 always; `for i in 0..v.len()` compiles. Collect the mutations and apply with `extend`.
5. **Move temporarily** with `Option::take` / `std::mem::take` when the field needs to leave and come back.
6. **Only then** `Rc<RefCell<_>>`/`Arc<Mutex<_>>`, and lastly `clone()`. Each step is more expensive than the previous one.

Places that already apply this: `app.rs:37` exits early with `let ... else` instead of nesting `match`; `rpc.rs:166` re-takes the lock in its own block so as not to hold the guard during the insertion.

## 3. Errors: `Result`, `?`, `thiserror` vs `anyhow`

`?` converts the error via `From`: the language reference defines `Result::Err(e)` as `Result::Err(From::from(e))` (Rust Reference, "try propagation operator"). That is what lets `?` map `io::Error` into your error type.

- **Library (another crate consumes): `thiserror`.** Derives `std::error::Error`/`Display`/`From` and keeps the enum nameable, so the consumer can `match` on it. It is the crate's proposal (docs.rs/thiserror).
- **Binary: `anyhow`.** The crate doc speaks of "error handling in Rust applications": `anyhow::Result<T>` without an error parameter and `.context()` to attach the "what" to the error of the "why".
- **`Box<dyn Error>`** only for 1–2 failure points with no `context()` to add. If `.map_err(|e| format!(...))` shows up in series, the cost has already passed `anyhow`.

Real case of this environment: a Tauri command **requires** a `serde::Serialize` error (v2.tauri.app/develop/calling-rust), and that is why `Result<_, String>` is on every `#[tauri::command]` (`rpc.rs:72`, `sessions.rs:102`) and even on an internal helper (`sessions.rs:35`). `String` serializes, but the frontend ends up matching message text — when it needs to react to distinct causes, use enum + `impl Serialize`, which is the pattern documented by Tauri.

```rust
// biblioteca: enum nomeável, causa preservada
#[derive(Debug, thiserror::Error)]
pub enum LoadError {
    #[error("config ausente: {path}")]
    Missing { path: String },
    #[error("io: {0}")]
    Io(#[from] std::io::Error),
    #[error("json invalido: {0}")]
    Json(#[from] serde_json::Error),
}

pub fn load(path: &str) -> Result<Cfg, LoadError> {
    let raw = std::fs::read_to_string(path)?; // io::Error -> LoadError
    Ok(serde_json::from_str(&raw)?)           // serde_json::Error -> LoadError
}
```
```rust
// binário: contexto na borda, `?` no meio
use anyhow::Context;

fn main() -> anyhow::Result<()> {
    let raw = std::fs::read_to_string("cfg.json").context("ler cfg.json")?;
    let cfg: Cfg = serde_json::from_str(&raw).context("parse cfg.json")?;
    println!("{}", cfg.n);
    Ok(())
}
```
Verified: compiles with `thiserror 2.0.21`, `anyhow 1.0.104`, edition 2021.

## 4. Types and traits: generics vs `dyn`

- **Generics/`impl Trait`** suffer monomorphization: one copy per concrete type, static dispatch and possible inlining. Cost: bigger binary and slower compilation (Rust Book ch. 18.2, which refers to ch. 10.1).
- **`dyn Trait`** uses dynamic dispatch through the trait object pointers at runtime, which prevents inlining and adds one indirection (Rust Book ch. 18.2). It is mandatory for a heterogeneous collection.
- **Criterion:** one implementor known at the call site → `impl Trait`; open set at runtime → `Box<dyn Trait>`; closed variants that need `match` → enum (cheaper than `dyn`).

Both sides appear here: `sessions.rs:18` receives `exists: impl Fn(&Path) -> bool` — test seam without `dyn` and without indirection; `app.rs:84` is forced to `&dyn egui::TextBuffer` because the signature is `egui`'s. In that conflict **the crate API wins**, not the preference.

Value Object has official backing: `C-NEWTYPE` (Rust API Guidelines) is newtype with no runtime cost. `RpcState(Mutex<HashMap<..>>)` (`rpc.rs:24`) is the first-class collection case — do not spread raw `HashMap` through the API. Large enum: lint `clippy::large_enum_variant`.

## 5. Concurrency: `Send`/`Sync`, `tokio` vs `std::thread`

`Send` = the value can be transferred to another thread; `Sync` = `&T` can be shared between threads (std docs). `thread::spawn` requires `F: FnOnce() -> T + Send + 'static` and `T: Send + 'static` (std docs) — that is what forces `move` and `Arc` in place of `Rc`.

- **`std::thread` for blocking work**: subprocess and line-by-line stdout reading. That is what `pi-deck` does — two threads per session, one per stream (`rpc.rs:140`, `rpc.rs:153`), emitting an event to the UI.
- **`tokio` for many concurrent I/O operations with waiting**: N connections, timeouts, `select!`. In a desktop app, prefer the runtime that already comes underneath (here `tauri` brings `tokio`) instead of opening a parallel `Runtime`.
- **`std::sync::Mutex` guard does not cross `.await`**: `MutexGuard` is `!Send` and the future stops being `Send` (verified: `future cannot be sent between threads safely ... await occurs here, with g maybe used later`). If the lock needs to cross the await, `tokio::sync::Mutex` — its guard is made for that (docs.rs/tokio Mutex).
- **Poison is part of the contract**: `lock()` returns `Result`. `if let Ok(..)` hides another thread's panic (`rpc.rs:29`); the explicit handling with a named error is in `rpc.rs:166`. Ignoring poison requires a comment saying why.

## 6. Prohibited idioms

- **`unwrap()`/`expect()` in a production path.** Use `?` or `unwrap_or_else` with a real fallback (`app.rs:19`). `.expect()` only where failing is the correct behavior — bootstrap (`main.rs:61`) and test (`sessions.rs:2038`). Lints: `clippy::unwrap_used`, `clippy::expect_used` (opt-in).
- **`clone()` to silence the borrow checker.** Legitimate when transferring ownership at an API boundary or for small data outside the hot path. Illegitimate inside `for`/`map`/render loop: `egui` redraws continuously and `app.rs:81` clones the `LayoutJob` every frame. Criterion: clone inside `ui`/`for`/`map` only with measurement.
- **`unsafe` without `// SAFETY:`** above it, naming the invariant the code keeps (`clippy::undocumented_unsafe_blocks`). Today: zero `unsafe` in both crates (grep).
- **`as` in a numeric cast that truncates** (`clippy::cast_possible_truncation`); use `try_into()`.
- **`panic!`/`unimplemented!()`/`todo!()`** in library code.

## 7. Anti-patterns with concrete symptom

| pattern | how it shows up in practice | fix |
|---|---|---|
| mutate a collection inside the loop that borrows it | `error[E0502]: cannot borrow v as mutable because it is also borrowed as immutable` | `for i in 0..v.len()`, or collect and `extend` |
| two live `&mut` at the same time | `error[E0499]: cannot borrow v as mutable more than once at a time` | one block/scope per borrow |
| `String` in a parameter that only reads | `error[E0382]: borrow of moved value: s` + rustc note suggesting borrow | `&str` in the parameter |
| return `&` of a local value | `error[E0597]: tmp does not live long enough` | return the owner (`String`/`Vec`) |
| `thread::spawn` closure capturing `Rc` | `error[E0277]: Rc<i32> cannot be sent between threads safely` | `Arc` + `Mutex`/`Atomic*` |
| `fn name() -> &str` without lifetime | `error[E0106]: missing lifetime specifier` | return `String` or annotate the real `'a` |
| `Mutex` guard crossing `.await` | `future cannot be sent between threads safely` / `MutexGuard ... not Send` | `tokio::sync::Mutex` or release the guard before |
| `#[derive(Clone)]` on everything | **no error** — only O(n) cost per call; the compiler stays quiet | derive only with proven need |
| `Result<_, String>` on every boundary | **no error** — the frontend compares `message` by substring | enum + `Serialize` at the edge |
| ignoring poison with `if let Ok` | **no error** — becomes silent partial state | `match` + named error |
| `impl Trait` returning a different type per branch | `error[E0308]: if and else have incompatible types` (rustc suggests changing the return to a trait object) | `Box<dyn Trait>` in the return |

The messages above were reproduced with `rustc 1.99.1`, edition 2021. The three lines marked **no error** do not break the compilation and are not caught by the default lints — only review catches them.

## Contested patterns (who wins)

| dispute | wins | why |
|---|---|---|
| `anyhow` also in a library | `thiserror` | the `anyhow` doc speaks of *applications*; the consumer loses `match` on the variants |
| `clone()` as a remedy for a borrow error | restructure | `clippy::redundant_clone` only detects provably redundant clone; the real cost is the "necessary" clone in the hot path, which no lint sees |
| `dyn` on everything vs generics on everything | the existing API when it fixes the shape (`egui`); otherwise `impl Trait` | `dyn` prevents inlining (Book ch. 18.2) |
| `std::sync::Mutex` vs `tokio::sync::Mutex` | `std` in synchronous code | async only wins when the guard crosses `.await` (tokio docs) |
| edition 2021 vs 2024 | 2021 in these crates | both declare `edition = "2021"` (`Cargo.toml:4`); changing edition is migration, not refactor — do not mix it inside the same crate |
| `Result<_, String>` in a Tauri command | enum + `Serialize`, with a test | recommendation of the official doc; it is conscious debt, not a pattern to copy |

## 8. References

- The Rust Book — ownership: https://doc.rust-lang.org/book/ch04-01-what-is-ownership.html
- The Rust Book — borrowing: https://doc.rust-lang.org/book/ch04-02-references-and-borrowing.html
- The Rust Book — `Result` and `?`: https://doc.rust-lang.org/book/ch09-02-recoverable-errors-with-result.html
- The Rust Book — trait objects, static vs dynamic dispatch: https://doc.rust-lang.org/book/ch18-02-trait-objects.html
- The Rust Book — threads: https://doc.rust-lang.org/book/ch16-01-threads.html
- The Rust Book — `Send`/`Sync`: https://doc.rust-lang.org/book/ch16-04-extensible-concurrency-sync-and-send.html
- The Rust Book — unsafe Rust: https://doc.rust-lang.org/book/ch20-01-unsafe-rust.html
- Rust Reference — lazy `?`/`From` handling and operators: https://doc.rust-lang.org/reference/expressions/operator-expr.html
- Rust API Guidelines: https://rust-lang.github.io/api-guidelines/ · type safety (`C-NEWTYPE`): https://rust-lang.github.io/api-guidelines/type-safety.html
- Clippy — lints: https://rust-lang.github.io/rust-clippy/master/index.html · usage/CI: https://doc.rust-lang.org/clippy/usage.html
- `std::marker::Send`: https://doc.rust-lang.org/std/marker/trait.Send.html · `Sync`: https://doc.rust-lang.org/std/marker/trait.Sync.html
- `thiserror`: https://docs.rs/thiserror/latest/thiserror/ · `anyhow`: https://docs.rs/anyhow/latest/anyhow/
- Tokio — tutorial: https://tokio.rs/tokio/tutorial · `sync::Mutex`: https://docs.rs/tokio/latest/tokio/sync/struct.Mutex.html
- Tauri v2 — commands and errors: https://v2.tauri.app/develop/calling-rust/
- Edition Guide: https://doc.rust-lang.org/edition-guide/rust-2021/index.html · 2024: https://doc.rust-lang.org/edition-guide/rust-2024/index.html

## What I could not verify

- Any **direct** use of `tokio` in both crates: none. In `phpide-rs`, `cargo tree -i tokio` on the host target prints nothing, but `--target all` would require downloading crates from iOS targets — the path of the `tokio 1.53.3` present in `Cargo.lock` was not traced.
- Whether `pi-deck` compiles and passes tests on this host: `src-tauri` depends on a desktop toolchain and I did not run build/test (forbidden to edit/generate artifact in the repo).
- Clippy is not installed in the local toolchain (`cargo-clippy` absent), so no lint cited was executed — the IDs were checked on the official lints page.
- `cargo tree` and `cargo test` ran only on throwaway crates in `/tmp`, not on the repositories.
- There is no CI configured in the two repositories that confirms `-D warnings` or `clippy::pedantic` in use.
