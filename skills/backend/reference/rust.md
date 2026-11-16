# Rust

> Referência de idiomas para código Rust neste ambiente. Estado: **ativo** (`skills/backend/reference/rust.md`).
> Base factual: os dois crates Rust do ambiente — `~/Projects/phpide-rs` e `~/Projects/pi-deck/src-tauri`. Toolchain local: `rustc 1.99.1`, `edition = "2021"` em ambos.

## Contexto verificado

| crate | deps diretas | async runtime |
|---|---|---|
| `phpide-rs` | `eframe`/`egui` 0.37, `tree-sitter` 0.27, `tree-sitter-php` 0.25 (`Cargo.toml:9-12`) | nenhum |
| `pi-deck` | `tauri` 2, `serde_json` 1, `base64` 0.21 (`Cargo.toml:11-13`) | nenhum direto; `tokio 1.54.1` só transitivo via `tauri 2.12.5` (`cargo tree -i tokio`) |

Consequência: **não há `tokio`, `clap`, `thiserror` nem `anyhow` como dependência direta**. Este guia descreve o que existe e o que adotar quando o problema aparecer; não prescreve stack nova.

## 1. Quando usar Rust aqui (e quando não)

Use quando ≥2 critérios abaixo forem verdadeiros e verificáveis:
1. **Entrega como binário nativo sem runtime externo.** Ambos são apps desktop; `pi-deck` até desliga o console no Windows (`src-tauri/src/main.rs:1`).
2. **Interop com biblioteca C/C++.** `tree-sitter` é consumido direto (`highlight.rs:9`), sem camada de binding própria.
3. **Memória/latência previsíveis, sem GC** — ownership libera no fim do escopo (Rust Book ch. 4.1).
4. **Processo longo com estado compartilhado** — `pi-deck` mantém `Mutex<HashMap<String, ChildHandle>>` com subprocessos vivos (`rpc.rs:24`).

Não use:
- **Sem gargalo medido.** Sem profiler ou alvo numérico de latência/memória, a reescrita é custo sem ganho mensurável.
- **Grafo com ciclos e ponteiro para "pai".** Exige `Rc<RefCell<_>>` + `Weak`, e `Rc` é `!Send` (std docs), o que trava o uso entre threads. Sem arena (`Vec` + índice) o borrow checker vira atrito diário.
- **CRUD/protótipo com prazo curto e stack TS/Go já instalada.** O tempo até o primeiro deploy é maior; segurança de memória não paga a diferença.
- **Equipe sem experiência e cronograma fechado.** O custo de aprender borrowing é real e cai nas primeiras semanas.

## 2. Ownership/borrowing

Três regras (Rust Book ch. 4.1, "Ownership Rules"): todo valor tem um dono; existe um dono por vez; ao sair do escopo o valor é liberado.

Padrão para "o borrow checker não deixa" — nesta ordem, pare no primeiro que resolve:

1. **Encurtar o empréstimo.** O borrow termina no último uso, não no fim do bloco; mover a mutação para depois resolve a maioria dos E0502 (verificado com `rustc 1.99.1`).
2. **Emprestar o campo, não o `self` inteiro.** `fn get(&self) -> &Vec<u8>` amarra a referência ao `self` todo; devolver `&[u8]` do campo ou separar em structs menores libera a mutação.
3. **Trocar posse por empréstimo na assinatura.** Em E0382 o próprio rustc emite `note: consider changing this parameter type ... to borrow`. `&str`/`&[T]` no parâmetro; `String`/`Vec<T>` só com posse real.
4. **Iterar diferente.** `for x in &v { v.push(..) }` é E0502 sempre; `for i in 0..v.len()` compila. Colete as mutações e aplique com `extend`.
5. **Mover temporariamente** com `Option::take` / `std::mem::take` quando o campo precisa sair e voltar.
6. **Só então** `Rc<RefCell<_>>`/`Arc<Mutex<_>>`, e por último `clone()`. Cada passo é mais caro que o anterior.

Locais que já aplicam isso: `app.rs:37` sai cedo com `let ... else` em vez de aninhar `match`; `rpc.rs:166` re-toma o lock em bloco próprio para não segurar o guard durante a inserção.

## 3. Erros: `Result`, `?`, `thiserror` vs `anyhow`

`?` converte o erro via `From`: a referência da linguagem define `Result::Err(e)` como `Result::Err(From::from(e))` (Rust Reference, "try propagation operator"). É isso que deixa `?` mapear `io::Error` no seu tipo de erro.

- **Biblioteca (outro crate consome): `thiserror`.** Deriva `std::error::Error`/`Display`/`From` e mantém o enum nomeável, então o consumidor consegue fazer `match`. É a proposta da crate (docs.rs/thiserror).
- **Binário: `anyhow`.** A doc da crate fala em "error handling in Rust applications": `anyhow::Result<T>` sem parâmetro de erro e `.context()` para anexar o "o quê" ao erro do "porquê".
- **`Box<dyn Error>`** só para 1–2 pontos de falha sem `context()` a acrescentar. Se aparecer `.map_err(|e| format!(...))` em série, o custo já passou do `anyhow`.

Caso real deste ambiente: comando Tauri **exige** erro `serde::Serialize` (v2.tauri.app/develop/calling-rust), e por isso `Result<_, String>` está em todo `#[tauri::command]` (`rpc.rs:72`, `sessions.rs:102`) e até em helper interno (`sessions.rs:35`). `String` serializa, mas o frontend passa a casar com texto de mensagem — quando precisar reagir a causas distintas, use enum + `impl Serialize`, que é o padrão documentado pelo Tauri.

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
Verificado: compila com `thiserror 2.0.21`, `anyhow 1.0.104`, edition 2021.

## 4. Tipos e traits: generics vs `dyn`

- **Generics/`impl Trait`** sofrem monomorphization: uma cópia por tipo concreto, dispatch estático e inlining possível. Custo: binário maior e compilação mais lenta (Rust Book ch. 18.2, que remete a ch. 10.1).
- **`dyn Trait`** usa dispatch dinâmico pelos ponteiros do trait object em runtime, o que impede inlining e adiciona uma indireção (Rust Book ch. 18.2). É obrigatório para coleção heterogênea.
- **Critério:** um implementador conhecido no call site → `impl Trait`; conjunto aberto em runtime → `Box<dyn Trait>`; variantes fechadas que precisam de `match` → enum (mais barato que `dyn`).

Os dois lados aparecem aqui: `sessions.rs:18` recebe `exists: impl Fn(&Path) -> bool` — seam de teste sem `dyn` e sem indireção; `app.rs:84` é forçado a `&dyn egui::TextBuffer` porque a assinatura é da `egui`. Nesse conflito **vence a API da crate**, não a preferência.

Value Object tem respaldo oficial: `C-NEWTYPE` (Rust API Guidelines) é newtype sem custo em runtime. `RpcState(Mutex<HashMap<..>>)` (`rpc.rs:24`) é o caso de coleção de primeira classe — não espalhar `HashMap` cru pela API. Enum grande: lint `clippy::large_enum_variant`.

## 5. Concorrência: `Send`/`Sync`, `tokio` vs `std::thread`

`Send` = o valor pode ser transferido para outra thread; `Sync` = `&T` pode ser compartilhado entre threads (std docs). `thread::spawn` exige `F: FnOnce() -> T + Send + 'static` e `T: Send + 'static` (std docs) — é o que obriga `move` e `Arc` no lugar de `Rc`.

- **`std::thread` para trabalho blocking**: subprocesso e leitura de stdout linha a linha. É o que `pi-deck` faz — duas threads por sessão, uma por stream (`rpc.rs:140`, `rpc.rs:153`), emitindo evento para a UI.
- **`tokio` para muitas operações de I/O concorrentes com espera**: N conexões, timeouts, `select!`. Em app desktop, prefira o runtime que já vem por baixo (aqui `tauri` traz `tokio`) em vez de abrir um `Runtime` paralelo.
- **Guard de `std::sync::Mutex` não atravessa `.await`**: `MutexGuard` é `!Send` e o futuro deixa de ser `Send` (verificado: `future cannot be sent between threads safely ... await occurs here, with g maybe used later`). Se o lock precisa cruzar o await, `tokio::sync::Mutex` — o guard dele é feito para isso (docs.rs/tokio Mutex).
- **Poison faz parte do contrato**: `lock()` devolve `Result`. `if let Ok(..)` esconde panic de outra thread (`rpc.rs:29`); o tratamento explícito com erro nomeado está em `rpc.rs:166`. Ignorar poison exige comentário dizendo por quê.

## 6. Idiomas proibidos

- **`unwrap()`/`expect()` em caminho de produção.** Use `?` ou `unwrap_or_else` com fallback real (`app.rs:19`). `.expect()` só onde falhar é o comportamento correto — bootstrap (`main.rs:61`) e teste (`sessions.rs:2038`). Lints: `clippy::unwrap_used`, `clippy::expect_used` (opt-in).
- **`clone()` para calar o borrow checker.** Legítimo ao transferir posse numa fronteira de API ou em dado pequeno fora de hot path. Ilegítimo dentro de `for`/`map`/render loop: `egui` redesenha continuamente e `app.rs:81` clona o `LayoutJob` a cada frame. Critério: clone dentro de `ui`/`for`/`map` só com medição.
- **`unsafe` sem `// SAFETY:`** acima, nomeando a invariante que o código mantém (`clippy::undocumented_unsafe_blocks`). Hoje: zero `unsafe` nos dois crates (grep).
- **`as` em cast numérico que trunca** (`clippy::cast_possible_truncation`); use `try_into()`.
- **`panic!`/`unimplemented!()`/`todo!()`** em código de biblioteca.

## 7. Anti-padrões com sintoma concreto

| padrão | como aparece na prática | correção |
|---|---|---|
| mutar coleção dentro do loop que a empresta | `error[E0502]: cannot borrow v as mutable because it is also borrowed as immutable` | `for i in 0..v.len()`, ou coletar e `extend` |
| dois `&mut` vivos ao mesmo tempo | `error[E0499]: cannot borrow v as mutable more than once at a time` | um bloco/escopo por borrow |
| `String` em parâmetro que só lê | `error[E0382]: borrow of moved value: s` + note do rustc sugerindo borrow | `&str` no parâmetro |
| devolver `&` de valor local | `error[E0597]: tmp does not live long enough` | devolver o dono (`String`/`Vec`) |
| closure de `thread::spawn` capturando `Rc` | `error[E0277]: Rc<i32> cannot be sent between threads safely` | `Arc` + `Mutex`/`Atomic*` |
| `fn name() -> &str` sem lifetime | `error[E0106]: missing lifetime specifier` | devolver `String` ou anotar o `'a` real |
| guard de `Mutex` atravessando `.await` | `future cannot be sent between threads safely` / `MutexGuard ... not Send` | `tokio::sync::Mutex` ou soltar o guard antes |
| `#[derive(Clone)]` em tudo | **sem erro** — só custo O(n) por chamada; compilador fica quieto | derivar só com necessidade provada |
| `Result<_, String>` em toda fronteira | **sem erro** — frontend compara `message` por substring | enum + `Serialize` na borda |
| ignorar poison com `if let Ok` | **sem erro** — vira estado parcial silencioso | `match` + erro nomeado |
| `impl Trait` retornando tipo diferente por branch | `error[E0308]: if and else have incompatible types` (o rustc sugere trocar o retorno por trait object) | `Box<dyn Trait>` no retorno |

As mensagens acima foram reproduzidas com `rustc 1.99.1`, edition 2021. As três linhas marcadas **sem erro** não quebram a compilação e não são pegas pelos lints padrão — só revisão pega.

## Padrões contestados (quem vence)

| disputa | vence | por quê |
|---|---|---|
| `anyhow` também em biblioteca | `thiserror` | a doc do `anyhow` fala em *applications*; o consumidor perde `match` nos variantes |
| `clone()` como remédio para borrow error | reestruturar | `clippy::redundant_clone` só detecta clone provadamente redundante; o custo real é o clone "necessário" em hot path, que nenhum lint vê |
| `dyn` em tudo vs generics em tudo | a API já existente quando ela fixa a forma (`egui`); senão `impl Trait` | `dyn` impede inlining (Book ch. 18.2) |
| `std::sync::Mutex` vs `tokio::sync::Mutex` | `std` em código síncrono | o async só ganha quando o guard cruza `.await` (tokio docs) |
| edition 2021 vs 2024 | 2021 nestes crates | ambos declaram `edition = "2021"` (`Cargo.toml:4`); trocar edition é migração, não refactor — não misture dentro do mesmo crate |
| `Result<_, String>` em comando Tauri | enum + `Serialize`, com teste | recomendação da doc oficial; é dívida consciente, não padrão a copiar |

## 8. Referências

- The Rust Book — ownership: https://doc.rust-lang.org/book/ch04-01-what-is-ownership.html
- The Rust Book — borrowing: https://doc.rust-lang.org/book/ch04-02-references-and-borrowing.html
- The Rust Book — `Result` e `?`: https://doc.rust-lang.org/book/ch09-02-recoverable-errors-with-result.html
- The Rust Book — trait objects, static vs dynamic dispatch: https://doc.rust-lang.org/book/ch18-02-trait-objects.html
- The Rust Book — threads: https://doc.rust-lang.org/book/ch16-01-threads.html
- The Rust Book — `Send`/`Sync`: https://doc.rust-lang.org/book/ch16-04-extensible-concurrency-sync-and-send.html
- The Rust Book — unsafe Rust: https://doc.rust-lang.org/book/ch20-01-unsafe-rust.html
- Rust Reference — lazy `?`/`From` handling e operadores: https://doc.rust-lang.org/reference/expressions/operator-expr.html
- Rust API Guidelines: https://rust-lang.github.io/api-guidelines/ · type safety (`C-NEWTYPE`): https://rust-lang.github.io/api-guidelines/type-safety.html
- Clippy — lints: https://rust-lang.github.io/rust-clippy/master/index.html · uso/CI: https://doc.rust-lang.org/clippy/usage.html
- `std::marker::Send`: https://doc.rust-lang.org/std/marker/trait.Send.html · `Sync`: https://doc.rust-lang.org/std/marker/trait.Sync.html
- `thiserror`: https://docs.rs/thiserror/latest/thiserror/ · `anyhow`: https://docs.rs/anyhow/latest/anyhow/
- Tokio — tutorial: https://tokio.rs/tokio/tutorial · `sync::Mutex`: https://docs.rs/tokio/latest/tokio/sync/struct.Mutex.html
- Tauri v2 — comandos e erros: https://v2.tauri.app/develop/calling-rust/
- Edition Guide: https://doc.rust-lang.org/edition-guide/rust-2021/index.html · 2024: https://doc.rust-lang.org/edition-guide/rust-2024/index.html

## O que não consegui verificar

- Qualquer uso de `tokio` **direto** em ambos os crates: nenhum. Em `phpide-rs`, `cargo tree -i tokio` no target host não imprime nada, mas `--target all` exigiria baixar crates de targets iOS — o caminho do `tokio 1.53.3` presente no `Cargo.lock` não foi rastreado.
- Se `pi-deck` compila e passa testes neste host: `src-tauri` depende de toolchain de desktop e não rodei build/test (proibido editar/gerar artefato no repo).
- Clippy não está instalado no toolchain local (`cargo-clippy` ausente), então nenhum lint citado foi executado — os IDs foram conferidos na página oficial de lints.
- `cargo tree` e `cargo test` rodaram apenas em crates descartáveis em `/tmp`, não nos repositórios.
- Não existe CI configurado nos dois repositórios que confirme `-D warnings` ou `clippy::pedantic` em uso.
