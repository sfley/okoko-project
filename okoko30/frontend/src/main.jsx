import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "react-grid-layout/css/styles.css";
import "react-resizable/css/styles.css";
import "./styles.css";
class ErrorBoundary extends React.Component {
  state = { error: false };
  static getDerivedStateFromError() {
    return { error: true };
  }
  render() {
    return this.state.error ? (
      <main className="fatal-error">
        <h1>Un instant, reprenons.</h1>
        <p>
          Une erreur d’affichage est survenue. Vos données restent conservées.
        </p>
        <button className="button" onClick={() => location.reload()}>
          Recharger l’application
        </button>
      </main>
    ) : (
      this.props.children
    );
  }
}
createRoot(document.getElementById("root")).render(
  <ErrorBoundary>
    <App />
  </ErrorBoundary>,
);
