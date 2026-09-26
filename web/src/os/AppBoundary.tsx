import { Component, type ErrorInfo, type ReactNode } from "react";

/**
 * AppBoundary keeps one app's crash inside its window: the rest of the
 * desktop, and the game, keep running.
 */
export class AppBoundary extends Component<
  { name: string; children: ReactNode },
  { error: Error | null }
> {
  state = { error: null as Error | null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(`${this.props.name} crashed`, error, info.componentStack);
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="empty">
        <p>
          <strong>{this.props.name}</strong> travou. O resto do computador continua funcionando.
        </p>
        <pre className="error-text">{this.state.error.message}</pre>
        <button type="button" className="btn small" onClick={() => this.setState({ error: null })}>
          Abrir de novo
        </button>
      </div>
    );
  }
}
