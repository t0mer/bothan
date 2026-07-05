export interface Host {
  id: number;
  hostname: string;
  enabled: boolean;
  publish: boolean;
  ignore_mismatch: boolean;
  from_cache: boolean;
  max_age_hours?: number;
  notes: string;
  created_at: string;
  updated_at: string;
}

export interface HostInput {
  hostname: string;
  publish?: boolean;
  ignore_mismatch?: boolean;
  from_cache?: boolean;
  max_age_hours?: number;
  notes?: string;
}

export interface Settings {
  server: {
    host: string;
    port: number;
    base_path: string;
    env_overridden: string[];
    restart_required: boolean;
  };
  log: { level: string; format: string };
  ssllabs: {
    api_version: string;
    email: string;
    poll_interval: string;
    max_workers: number;
    scan_timeout: string;
    default_publish: boolean;
  };
  metrics: { enabled: boolean; restart_required: boolean };
  bootstrap: { database_path: string; encryption_key_set: boolean };
}

export interface SettingsPatch {
  server?: Partial<{ host: string; port: number; base_path: string }>;
  log?: Partial<{ level: string; format: string }>;
  ssllabs?: Partial<{
    api_version: string;
    email: string;
    poll_interval: string;
    max_workers: number;
    scan_timeout: string;
    default_publish: boolean;
  }>;
  metrics?: Partial<{ enabled: boolean }>;
}
