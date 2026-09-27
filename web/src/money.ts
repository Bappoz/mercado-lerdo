// Money in the game is whole reais (internal/career.Money): R$ 1.500, -R$ 150.
const brl = new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 0 });

export function money(v: number): string {
  return `${v < 0 ? "-" : ""}R$ ${brl.format(Math.abs(v))}`;
}
