import { useState } from "react";
import type { GameState } from "../api.ts";
import {
  type Coat,
  cssVars,
  type Prefs,
  slotthyColors,
  type TextSize,
  THEMES,
  type Theme,
} from "../theme.ts";
import { WALLPAPERS } from "../wallpaper.ts";

type Section = "aparencia" | "sistema";

function Choice<T extends string | boolean>({
  label,
  value,
  options,
  onPick,
}: {
  label: string;
  value: T;
  options: { v: T; label: string; swatch?: string }[];
  onPick: (v: T) => void;
}) {
  return (
    <fieldset className="choice">
      <legend>{label}</legend>
      <div className="chips">
        {options.map((o) => (
          <button
            type="button"
            key={String(o.v)}
            className="chip"
            aria-pressed={value === o.v}
            onClick={() => onPick(o.v)}
          >
            {o.swatch && <span className="swatch" style={{ background: o.swatch }} />}
            {o.label}
          </button>
        ))}
      </div>
    </fieldset>
  );
}

/** A tiny desktop drawn with the theme's own tokens. */
function Preview({ theme, prefs }: { theme: Theme; prefs: Prefs }) {
  const vars = cssVars({ ...prefs, theme: theme.id, accent: 0 });
  const bar = theme.titleBar(theme.accents[0], "inbox");
  return (
    <span className="mini" style={vars as React.CSSProperties} aria-hidden="true">
      <span className="mini-win">
        <span className="mini-bar" style={{ background: bar.bg }} />
      </span>
      <span className="mini-term" />
      <span className="mini-dock" />
    </span>
  );
}

export function Settings({
  prefs,
  setPrefs,
  state,
  busy,
  onStop,
  onReplayTour,
}: {
  prefs: Prefs;
  setPrefs: (p: Prefs) => void;
  state: GameState;
  busy: boolean;
  onStop: (down: boolean) => void;
  onReplayTour: () => void;
}) {
  const [section, setSection] = useState<Section>("aparencia");
  const theme = THEMES.find((t) => t.id === prefs.theme) ?? THEMES[0];
  const set = <K extends keyof Prefs>(k: K, v: Prefs[K]) => setPrefs({ ...prefs, [k]: v });
  const coats: { v: Coat; label: string }[] = [
    { v: "castanho", label: "Castanho" },
    { v: "cinza", label: "Cinza" },
    { v: "mel", label: "Mel" },
  ];

  return (
    <div className="settings">
      <nav className="settings-nav" aria-label="Seções">
        <button
          type="button"
          aria-current={section === "aparencia" ? "page" : undefined}
          onClick={() => setSection("aparencia")}
        >
          Aparência
        </button>
        <button
          type="button"
          aria-current={section === "sistema" ? "page" : undefined}
          onClick={() => setSection("sistema")}
        >
          Sistema
        </button>
      </nav>

      {section === "aparencia" ? (
        <div className="settings-body">
          <fieldset className="choice">
            <legend>Tema</legend>
            <div className="themes">
              {THEMES.map((t) => (
                <button
                  type="button"
                  key={t.id}
                  className="theme-card"
                  aria-pressed={prefs.theme === t.id}
                  onClick={() => setPrefs({ ...prefs, theme: t.id, accent: 0 })}
                >
                  <Preview theme={t} prefs={prefs} />
                  <strong>{t.name}</strong>
                  <span className="muted">{t.blurb}</span>
                </button>
              ))}
            </div>
          </fieldset>
          <fieldset className="choice">
            <legend>Papel de parede</legend>
            <div className="walls">
              {WALLPAPERS.map((w) => (
                <button
                  type="button"
                  key={w.id}
                  className="wall-card"
                  aria-pressed={prefs.wallpaper === w.id}
                  onClick={() => set("wallpaper", w.id)}
                >
                  <span className="wall-thumb" aria-hidden="true">
                    <span
                      className="wall-desk"
                      style={cssVars({ ...prefs, wallpaper: w.id }) as React.CSSProperties}
                    />
                  </span>
                  {w.name}
                </button>
              ))}
            </div>
          </fieldset>
          <div className="settings-grid">
            <fieldset className="choice">
              <legend>Cor de destaque</legend>
              <div className="chips">
                {theme.accents.map((a, i) => (
                  <button
                    type="button"
                    key={a.name}
                    className="swatch-btn"
                    aria-label={`Destaque ${a.name}`}
                    title={a.name}
                    aria-pressed={prefs.accent === i}
                    style={{ background: a.hex }}
                    onClick={() => set("accent", i)}
                  />
                ))}
              </div>
            </fieldset>
            <Choice<"pixel" | "suave">
              label="Moldura"
              value={prefs.chrome}
              onPick={(v) => set("chrome", v)}
              options={[
                { v: "pixel", label: "Pixelada" },
                { v: "suave", label: "Suave" },
              ]}
            />
            <Choice<TextSize>
              label="Tamanho do texto"
              value={prefs.text}
              onPick={(v) => set("text", v)}
              options={[
                { v: "P", label: "P" },
                { v: "M", label: "M" },
                { v: "G", label: "G" },
              ]}
            />
            <Choice<Coat>
              label="Pelagem da Slotthy"
              value={prefs.coat}
              onPick={(v) => set("coat", v)}
              options={coats.map((c) => ({
                ...c,
                swatch: slotthyColors({ ...prefs, coat: c.v }).o,
              }))}
            />
            <Choice<boolean>
              label="Animações"
              value={prefs.motion}
              onPick={(v) => set("motion", v)}
              options={[
                { v: true, label: "Ligadas" },
                { v: false, label: "Desligadas" },
              ]}
            />
          </div>
          <div className="actions">
            <button type="button" className="btn small ghost" onClick={onReplayTour}>
              Rever o treinamento do primeiro dia
            </button>
          </div>
          <p className="muted small">
            As preferências ficam neste computador (~/.config/sesim/ui.json). Com &ldquo;reduzir
            movimento&rdquo; ligado no sistema, as animações ficam desligadas de qualquer jeito.
          </p>
        </div>
      ) : (
        <div className="settings-body">
          <dl className="facts">
            <dt>Cenário</dt>
            <dd>
              {state.scenario.title}{" "}
              <span className="muted">
                ({state.scenario.id} v{state.scenario.version})
              </span>
            </dd>
            <dt>Workspace</dt>
            <dd>
              <code>{state.session?.workspace ?? "ainda não criado"}</code>
            </dd>
            <dt>Loja</dt>
            <dd>{state.running ? "no ar" : "desligada"}</dd>
          </dl>
          <div className="actions">
            <button
              type="button"
              className="btn ghost"
              disabled={busy || !state.running}
              onClick={() => onStop(false)}
            >
              Pausar o tráfego
            </button>
            <button
              type="button"
              className="btn ghost"
              disabled={busy || !state.running}
              onClick={() => onStop(true)}
            >
              Desligar a loja
            </button>
          </div>
          <p className="muted small">
            Desligar apaga a sandbox (os dados voltam ao início quando ela subir de novo). O seu
            workspace e o progresso ficam. Fechar o sesim no terminal não desliga nada.
          </p>
        </div>
      )}
    </div>
  );
}
