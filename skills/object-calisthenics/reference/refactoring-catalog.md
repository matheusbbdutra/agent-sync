# Refactoring catalog

Source: Martin Fowler's *Catalog of Refactorings* (`refactoring.com/catalog`), free and author-published.
Names are canonical; wording here is rewritten. This is a **working subset** (~25 of the catalog), not
the complete list — open the site when you need the rest.

Never invent a refactoring name. If the move is not in the catalog, describe the change instead.

## Smell → move

Read the smell, then pick the smallest move that removes it. Apply one at a time, with tests green.

| Smell you can point at | Move | Notes |
|---|---|---|
| Long method doing several things | **Extract Method** | Name it for the intent, not the steps. Stop when the name would be "and" |
| Method with a dead-wrong body | **Inline Method** | Useful before extracting a better seam |
| `x = a.b().c()` repeated | **Extract Variable** | Same intent as Extract Method, at expression level |
| Variable assigned in several branches | **Split Variable** | One variable = one responsibility |
| Temp used in a single expression | **Inline Variable** then extract | Removes a name that carries nothing |
| Temp depending on other temps | **Replace Temp with Query** | Careful: query must be side-effect free |
| Assignment to a parameter | **Remove Assignments to Parameters** | Introduce a local instead |
| 10 parameters, always the same 3 together | **Introduce Parameter Object** | If those 3 already exist as a type, pass the type |
| Caller passes values the method can obtain itself | **Preserve Whole Object** / **Replace Parameter with Query** | Reduces arity without hiding a dependency |
| Boolean parameter switching behaviour | **Remove Flag Argument** | Two methods beat one flag |
| Nested `if/else` where the early case exits | **Replace Nested Conditional with Guard Clauses** | Flattens the happy path |
| `if` with several conditions tested together | **Decompose Conditional** | Name the condition |
| Same condition repeated | **Consolidate Conditional Expression** | One named predicate |
| Same `if` body in several places | **Consolidate Duplicate Conditional Fragments** | Move the common part out |
| Conditional on type code / enum in many methods | **Replace Conditional with Polymorphism** | The `switch` disappears; the variation gets a home |
| `if (x != null)` everywhere | **Introduce Special Case** (Null Object) | Only when "no value" is a real domain concept |
| Boolean loop control variable | **Remove Control Flag** | `break`/`return` expresses intent |
| Method uses data from another object more than its own | **Move Method** | Then fix the name |
| Field used mostly by another class | **Move Field** | Check the invariant written by the callers |
| One class doing two jobs | **Extract Class** | Split by data the methods touch, not by method count |
| Class barely used | **Inline Class** | Fold it into the only consumer |
| `a.getB().getC().doIt()` | **Hide Delegate** | Caller reaches through two objects |
| Delegate methods everywhere | **Remove Middle Man** | The cost of hiding exceeded the cost of chaining |
| Magic number / literal sprinkled around | **Replace Magic Literal with Symbolic Constant** | Named constant carries meaning |
| Collection exposed as list | **Encapsulate Collection** | Read-only view + add/remove methods |
| Getter/setter only, public field | **Encapsulate Record** / **Encapsulate Field** | Enables invariant later |
| Primitive used as a type (currency, email, id) | **Replace Primitive with Object** | Small type = validation point |
| Type code with behaviour | **Replace Type Code with Subclasses** / **with State** | Choose by whether behaviour or lifecycle varies |
| Type code with behaviour for a bounded set of cases | **Replace Type Code with State/Strategy** | Composition preferred when cases multiply |
| Subclass used only for data variance | **Replace Subclass with Delegate** | Inheritance is for behaviour |
| Class hierarchy with repeated method | **Pull Up Method** / **Pull Up Field** | Remove the duplicate before adding a case |
| Method only one subclass uses | **Push Down Method** | Keeps base minimal |
| Class with several unrelated clients | **Extract Superclass** | If the shared part has a name in the domain |
| Subclass is the only useful one | **Collapse Hierarchy** | Removes ceremony |
| Callers use a method differently than intended | **Rename Method** | Cheapest refactoring with the highest return |
| Ugly sequence behind every call | **Replace Constructor with Factory Function** / **Replace Error Code with Exception** | Signals intent at the boundary |
| Working on the whole object | **Change Value to Reference** / **Change Reference to Value** | Depends on identity being meaningful |
| Two-way association nobody uses both ways | **Change Bidirectional Association to Unidirectional** | Half the coupling, none of the invariant work |

## Grouping, for recall

The catalog groups moves by intent — useful when you know the pain but not the name:

- **Composing methods:** Extract Method · Inline Method · Extract Variable · Inline Variable ·
  Replace Temp with Query · Split Variable · Remove Assignments to Parameters ·
  Replace Method with Method Object · Substitute Algorithm
- **Moving features:** Move Method · Move Field · Extract Class · Inline Class · Hide Delegate ·
  Remove Middle Man · Introduce Foreign Method · Introduce Local Extension
- **Organizing data:** Encapsulate Record · Encapsulate Collection · Replace Primitive with Object ·
  Replace Magic Literal · Replace Type Code with Subclasses · Replace Subclass with Delegate
- **Simplifying conditionals:** Decompose Conditional · Consolidate Conditional Expression ·
  Replace Nested Conditional with Guard Clauses · Replace Conditional with Polymorphism ·
  Introduce Special Case · Remove Control Flag
- **Simplifying method calls:** Rename Method · Preserve Whole Object · Replace Parameter with
  Query · Introduce Parameter Object · Remove Flag Argument · Replace Constructor with Factory
  Function · Replace Error Code with Exception
- **Dealing with generalization:** Pull Up Method · Pull Up Field · Push Down Method ·
  Extract Superclass · Collapse Hierarchy · Replace Inheritance with Delegation

## Go-specific moves that survive review

Refactoring a Go codebase has its own cheap wins — apply the same discipline (one move, tests green):

| Smell | Move |
|---|---|
| `if err != nil { return err }` after wrapping adds no context | annotate once at the boundary, not at every hop |
| Interface declared next to its implementation | declare at the consumer, with the smallest method set |
| Same struct passed to 6 methods | pass the fields it actually reads |
| `interface{}`/`any` in a signature | concrete type; generics when the algorithm is truly type-agnostic |
| 5-deep `map[string]interface{}` | **Replace Primitive with Object**, then validate at construction |
| `init()` doing work | constructor returning `(T, error)` |
| Mutex held across a call | narrow the critical section before anything else |
| Package-level variable used as cache | explicit dependency; global state is the smell |

## When NOT to refactor

- **No test (or no way to observe behaviour):** write the characterization test first. Refactoring
  without a safety net is rewriting with extra steps.
- **You cannot describe the change as a sequence of named moves:** you are redesigning, not
  refactoring — that needs its own decision and its own review.
- **The code is about to be deleted or replaced:** effort spent on dead code is negative.
- **The branch is already large:** land the current change first; a refactor bundled with a feature
  makes both unreviewable.
- **The rule is enforced by tooling:** formatting, import order, and lint fixes belong to `gofmt` and
  the linter, not to a hand-edited diff.

## Safety protocol

1. Run the test suite; record the result before touching anything.
2. Apply **one** move. Compile. Run tests.
3. Commit the move alone, with the refactoring name in the message (`refactor(money): replace
   primitive with object`).
4. Repeat. If a move needs three others to compile, you mis-scoped it — pick a smaller one.
5. If tests fail and the fix is not mechanical, revert the move instead of debugging forward: the
   point of the catalog is that each move is small enough to throw away.
