package topology

import "slices"

// Kind is a catalog component.
type Kind string

// Catalog kinds.
const (
	KindNginx     Kind = "nginx"
	KindApp       Kind = "app"
	KindPostgres  Kind = "postgres"
	KindPgBouncer Kind = "pgbouncer"
)

// Tier is what one size of a component gets and costs.
type Tier struct {
	CPUs   float64 `json:"cpus"`
	Memory int64   `json:"memory"` // bytes
	Price  float64 `json:"price"`  // modeled US$ per month, per instance
	// MaxConnections is how many connections this size has memory for
	// (postgres only).
	MaxConnections int `json:"max_connections,omitempty"`
}

// Setting documents one field of a component in topology.yaml.
type Setting struct {
	Field string `json:"field"`
	Range string `json:"range"`
	Doc   string `json:"doc"`
}

// Entry is one component of the catalog, with the docs the player reads in
// the Docs app. The docs explain what a tool is and what it costs, the way
// real documentation would; they never say which scenario it solves.
type Entry struct {
	Kind      Kind          `json:"kind"`
	Name      string        `json:"name"`
	Optional  bool          `json:"optional"` // can be installed or removed
	Summary   string        `json:"summary"`
	WhenToUse string        `json:"when_to_use"`
	Tradeoffs string        `json:"tradeoffs"`
	Settings  []Setting     `json:"settings"`
	Sizes     map[Size]Tier `json:"sizes"`
	Snippet   string        `json:"snippet"` // what Install adds to topology.yaml
}

const mib = int64(1) << 20

// Catalog is every component a topology may use. Sizes are scaled physics:
// the S tiers are the calibrated Stage 1 machine (docs/architecture/sandbox.md §7).
var Catalog = map[Kind]Entry{
	KindNginx: {
		Kind: KindNginx, Name: "Nginx",
		Summary:   "Porta de entrada: recebe todo o tráfego dos clientes e repassa para as instâncias do app (proxy reverso e balanceador).",
		WhenToUse: "Sempre está lá. A configuração fica em `nginx/default.conf` no workspace: upstreams, timeouts, tentativas em outra instância, cache.",
		Tradeoffs: "Uma configuração inválida derruba a entrada inteira; o deploy testa a configuração (`nginx -t`) antes de trocar.",
		Settings:  []Setting{{"size", "S | M", "CPU e memória do proxy."}},
		Sizes: map[Size]Tier{
			S: {CPUs: 0.5, Memory: 64 * mib, Price: 10},
			M: {CPUs: 1, Memory: 128 * mib, Price: 20},
		},
	},
	KindApp: {
		Kind: KindApp, Name: "App Nexora",
		Summary:   "O serviço Go do seu workspace. Cada réplica é uma instância igual, atrás do Nginx.",
		WhenToUse: "Mais réplicas dividem o trabalho do app e sobrevivem à perda de uma instância. Não ajudam se o gargalo está no banco.",
		Tradeoffs: "Cada réplica custa o preço do tamanho e abre o próprio pool de conexões com o banco: réplicas × `dbMaxConns` conexões no total.",
		Settings: []Setting{
			{"size", "S | M | L", "CPU e memória de cada réplica."},
			{"replicas", "1–4", "Quantas instâncias rodam."},
			{"database", "postgres | pgbouncer", "Para onde o app conecta."},
			{"dbMaxConns", "1–100", "Tamanho do pool de conexões de cada réplica (NEXORA_DB_MAX_CONNS)."},
		},
		Sizes: map[Size]Tier{
			S: {CPUs: 0.5, Memory: 128 * mib, Price: 20},
			M: {CPUs: 1, Memory: 256 * mib, Price: 40},
			L: {CPUs: 2, Memory: 512 * mib, Price: 80},
		},
	},
	KindPostgres: {
		Kind: KindPostgres, Name: "PostgreSQL",
		Summary:   "O banco de dados, fonte da verdade de produtos, usuários e pedidos.",
		WhenToUse: "Sempre está lá. Um tamanho maior dá mais CPU e memória a todas as consultas ao mesmo tempo.",
		Tradeoffs: "É o componente mais caro. Cada conexão aberta ocupa memória no servidor; `maxConnections` alto sem memória para acompanhar vira outro problema.",
		Settings: []Setting{
			{"size", "S | M | L", "CPU e memória do banco."},
			{"maxConnections", "20 até o teto do tamanho", "Máximo de conexões simultâneas (max_connections). Acima disso, o banco recusa novas conexões (\"too many clients\"). Cada conexão ocupa memória: S aguenta até 50, M até 150, L até 400."},
		},
		Sizes: map[Size]Tier{
			S: {CPUs: 0.5, Memory: 256 * mib, Price: 40, MaxConnections: 50},
			M: {CPUs: 1, Memory: 512 * mib, Price: 80, MaxConnections: 150},
			L: {CPUs: 2, Memory: 1024 * mib, Price: 160, MaxConnections: 400},
		},
	},
	KindPgBouncer: {
		Kind: KindPgBouncer, Name: "PgBouncer", Optional: true,
		Summary:   "Pooler de conexões: muitos clientes compartilham poucas conexões reais com o Postgres.",
		WhenToUse: "Quando o número de conexões dos clientes, e não o trabalho das consultas, é o que pressiona o banco. Para usar, aponte o app para ele (`app.database: pgbouncer`).",
		Tradeoffs: "Mais um componente no caminho de cada consulta. No modo `transaction`, o que depende da sessão (`SET` fora de transação, advisory locks de sessão, `LISTEN`) deixa de funcionar como antes. Inspecione com `psql -h pgbouncer -p 6432 pgbouncer -c 'SHOW POOLS'`.",
		Settings: []Setting{
			{"size", "S", "CPU e memória do pooler."},
			{"poolMode", "transaction | session", "transaction: a conexão real volta ao pool no fim de cada transação. session: fica com o cliente até ele desconectar."},
			{"defaultPoolSize", "1–200", "Conexões reais com o Postgres por banco/usuário."},
			{"maxClientConn", "10–2000", "Conexões de clientes que o PgBouncer aceita."},
		},
		Sizes: map[Size]Tier{
			S: {CPUs: 0.25, Memory: 32 * mib, Price: 8},
		},
		Snippet: "pgbouncer:\n  size: S\n  poolMode: transaction\n  defaultPoolSize: 20\n  maxClientConn: 500\n",
	},
}

func (e Entry) sizeNames() []Size {
	var out []Size
	for s := range e.Sizes {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

// Installed reports whether the optional kind is in t.
func (t Topology) Installed(k Kind) bool {
	switch k {
	case KindPgBouncer:
		return t.Components.PgBouncer != nil
	default:
		return true
	}
}
