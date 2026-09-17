# ReactorLab frontend

The public `/guest` route loads only ReactorLab's same-origin
`/api/v1/guest/resources` aggregate. It renders MiniDeploy-approved deployment
cards on the left and MiniBase-approved database cards on the right, stacking
them in that order on narrow screens.

Deployment links use only the guest-safe `url` supplied by MiniDeploy.
Database cards are informational. Each section handles loading, empty, and
upstream-unavailable states independently; private telemetry is never rendered
on the Guest page.

# React + Vite

This template provides a minimal setup to get React working in Vite with HMR and some Oxlint rules.

Currently, two official plugins are available:

- [@vitejs/plugin-react](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react) uses [Oxc](https://oxc.rs)
- [@vitejs/plugin-react-swc](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react-swc) uses [SWC](https://swc.rs/)

## React Compiler

The React Compiler is not enabled on this template because of its impact on dev & build performances. To add it, see [this documentation](https://react.dev/learn/react-compiler/installation).

## Expanding the Oxlint configuration

If you are developing a production application, we recommend using TypeScript with type-aware lint rules enabled. Check out the [TS template](https://github.com/vitejs/vite/tree/main/packages/create-vite/template-react-ts) for information on how to integrate TypeScript and Oxlint's TypeScript related rules in your project.
