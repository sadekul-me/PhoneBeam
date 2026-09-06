/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_COORDINATOR_ORIGIN?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
