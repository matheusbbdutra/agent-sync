# Reading a dependency graph

Reading criterion for a repository's import/call graph: what can be asserted, what needs to be
certified and what cannot be promised. This file **does not** define how to compute: computing belongs
to `repo-map`.

## 1. Division of labor: computing in `repo-map`, criterion here

- The binary exposes today, according to `repo-map --help`: `--update` (incremental cache), `--focus
  <path>` (subgraph), `--brief <path>` (delimited JSON), `--summary` (call/import hubs) and
  `--audit-removal <target>`; with `-depth`, `-max-tokens` and `-root` as adjustments.
- If the number you need (components, cut, cycle) does not come out of those modes, compute it outside
  and validate the result with the tests of section 2 — do not present the number without passing them.
- What this reference governs: which assertion the report can make with the graph that `repo-map`
  produced (connectivity, component, coupling, cycle, partition, optimality).

## 2. Validate the graph before concluding anything

- **Normalize to a simple graph.** Collapse duplicate edges between the same pair of vertices and
  discard loops (`v->v`, e.g.: recursive function) before counting degree, component or cycle: the
  book's model is `A ⊆ V(2)`, with no repeated edge and no coinciding endpoint (p.8).
- **Do not count an import line as an edge.** Two references between the same two modules are one
  edge; counting each line inflates degree, inflates the cycle count and creates a module "cycle" with
  itself (p.8).
- **Run the integrity test of the degrees:** `Σ g(v) = 2·|A|` (p.14) and the number of vertices of odd
  degree is even (p.15). Failure in either one = truncated export, duplicate edge or nonexistent
  endpoint.
- **Abort instead of reporting.** A conclusion drawn from a corrupted graph ("no cycles", "no
  dependencies") is worse than absence of conclusion (p.14-15).
- **Start with the empty and total cut:** `∇(∅) = ∇(V(G)) = ∅` (p.14). If your boundary computation
  returns an edge for those two cases, the export is wrong.

## 3. Components and connectivity

- **Certify connectivity by exhaustive reach**, never by sampling: traversal from a seed vertex and
  count equal to `n`. The equivalent criterion is `∇(X) ≠ ∅` for every proper and non-empty subset `X`
  (p.18).
- **Do not use 2-3 example paths as proof.** Entrypoint -> module A -> B does not cover islands
  (scripts, tests, dead code) that stay out of reach (p.18).
- **Decompose into components before ordering, scheduling or measuring impact.** A component is the
  maximal connected subgraph; each vertex belongs to exactly one; the graph is connected if and only
  if it has a single component (p.18).
- **Order and parallelize by component.** There is no order that crosses disconnected components:
  emitting a single global priority mixes independent islands that could run in parallel (p.18).
- **Treat "3 components" as an architecture finding**, not as a footnote (p.18).

## 4. Coupling by the cut, not by the total count

- **Compute `|∇(X)|`** = edges with exactly one endpoint in `X` and the other outside; edges internal
  to `X` do not enter (p.14).
- **Report that number as the coupling of the boundary.** Two cuts of different partitions become
  comparable by cardinality (p.14).
- **Do not elect the "most coupled module" by absolute fan-in/fan-out:** a large module always wins,
  and an internal import counted as external makes any partition look equally bad (p.14).

## 5. Cycles and redundant edges

- **Removal test:** an edge `a` takes part in a cycle if, and only if, `G − a` remains connected in the
  component where it lives (p.18).
- **Useful equivalence for "no cycle":** the graph is a forest if, and only if, each edge is a cut
  (p.17); connected and forest if, and only if, `G − a` stops being connected for every edge (p.18).
- **Cheap cycle budget:** `m ≥ n − c`, with `c` = number of components (p.18). `m − n + c` growing
  when adding an edge that does not change connectivity is a sign of a cycle.
- **Do not sell `m − n + c` as the number of simple cycles:** it is a bound/dimension, not an
  enumeration (p.18; the digest itself marks that reading as medium confidence).
- **Before keeping a dependency, ask whether it is a cut.** An edge that leaves without disconnecting
  is surplus for connectivity: treat it as a removal candidate, not as a necessary dependency
  (p.17-18).

## 6. Partition into 2 layers: demand a certificate

- **To assert that it works:** display the full bicoloring. Bicolorable if and only if it has no odd
  circuit (p.33).
- **To assert that it does not work:** display a single concrete odd circuit. "O unico motivo para a
  inexistencia de uma bicoloracao e a presenca de um circuito impar" (p.34) [the only reason for the
  nonexistence of a bicoloring is the presence of an odd circuit], and the odd circuit is "um
  certificado simples e convincente" (p.34) [a simple and convincing certificate].
- **A partial bicoloring does not serve** as proof of layering; it does not cover all the vertices
  (p.33-34).
- **In a connected component the bicoloring is unique** (p.35): if you found two distinct ones, there
  is more than one component — go back to section 3.

## 7. What the report can promise

| Report question | Can assert | Basis |
| --- | --- | --- |
| "that module is the most coupled one" | only with `|∇(X)|` of the proposed boundary | p.14 |
| "the repo is a connected graph" | only with exhaustive reach = `n` | p.18 |
| "that import is redundant" | only with the removal test | p.18 |
| "it can be split into 2 layers" | only with a full bicoloring | p.33-35 |
| "it can be split into 2 layers" (negative) | only with an odd circuit displayed | p.34 |
| "that is the minimal set of files to review" | no, unless proven: maximum stable set is `2^n` | p.28 |

- **Label a heuristic as maximal.** Every maximal stable set has at least `n/(Δ+1)` vertices
  (p.21); that is a proportional guarantee, not optimality. In the matching case, finding a maximal one
  is easy and looking for the maximum is "much harder" (p.37).
- **Classify tractability before promising optimum:** maximum stable set and minimum coloring cost
  `2^n` (p.28, p.36); exact isomorphism costs `n!` (p.13); deciding whether the chromatic index is
  `Δ` or `Δ+1` has no known fast algorithm (p.53).
- **What is polynomial in the book:** maximum matching (p.47) and, in a bipartite graph, maximum
  matching equal to minimum cover (König, p.41).
- **Without a cheap certificate, say so:** outside the bicolorable case no substantially better
  certificate is known (p.36).

## 8. Limit of the model: undirected graph

- The book deals exclusively with simple **undirected** and finite graphs (p.8). It answers "who is
  linked to whom", not "who depends on whom".
- **Topological ordering, DAG and critical path are not in this source** (verified in the Portuguese digest: zero
  occurrences of `topológ`, `acícl`, `DAG`, `direcionad`, `digrafo`, `orientad`). Do not use p.N of
  this book to support build order or critical path.
- Practical consequence: dependency direction, directed fan-in/fan-out and sequencing stay outside
  what this reference can assert.

## 9. Checklist before writing the finding

- [ ] Graph normalized (no duplicate edge, no loop) (p.8).
- [ ] `Σ g(v) = 2·|A|` and parity of the degrees checked (p.14-15).
- [ ] Components computed and reported as a unit (p.18).
- [ ] Connectivity certified by reach, not by example (p.18).
- [ ] Coupling measured by `|∇(X)|` (p.14).
- [ ] Cycle proven by removal, not by suspicion (p.18).
- [ ] Layering with certificate (bicoloring or odd circuit) (p.33-34).
- [ ] Heuristic labeled as maximal, with a bound (p.21, p.37).
- [ ] No assertion of order/critical path attributed to this source (p.8).

## Sources

- Uma Introducao Sucinta a Teoria dos Grafos (P. Feofiloff, Y. Kohayakawa, Y. Wakabayashi) — ch. 1
  (p.8: simple undirected graph, with no repeated edge and no loop; p.13: isomorphism `n!`; p.14:
  cut `∇(X)`, identity `Σ g(v) = 2·|A|`; p.15: parity of the degrees; p.17: forest and cut edge;
  p.18: connectivity, components, removal test, `m ≥ n − c`); ch. 2 (p.21: `α(G) ≥
  n/(Δ+1)`; p.28: maximum stable set in `2^n`); ch. 3 (p.33-35: bicoloring, odd circuit as
  certificate, uniqueness in a connected component; p.36: absence of a cheap certificate); ch. 4 (p.37:
  maximal vs maximum; p.41: König; p.47: maximum matching polynomial); ch. 5 (p.53: decision `Δ`
  vs `Δ+1` without a fast algorithm).
- Tool: `repo-map --help` (modes and flags cited in section 1), run in the local environment; the
  binary does not document computation of components, cut or cycle, which is why section 1 separates
  computing from criterion.
