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
  latest_grade?: string;
  last_scan_status?: string;
  last_scan_at?: string;
}

export interface Schedule {
  id: number;
  name: string;
  spec: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface Channel {
  id: number;
  name: string;
  type: string;
  needs_credentials: boolean;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface Rule {
  id: number;
  host_id?: number | null;
  name: string;
  condition_type: string;
  threshold_grade?: string;
  expiry_days?: number;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface Scan {
  id: number;
  host_id: number;
  status: string;
  trigger: string;
  overall_grade: string;
  engine_version: string;
  criteria_version: string;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  endpoints?: ScanEndpoint[];
}

export interface ScanRef {
  id: number;
  grade: string;
  status: string;
  created_at: string;
}

export interface EndpointDiff {
  ip_address: string;
  change: string;
  from_grade?: string;
  to_grade?: string;
  grade_changed: boolean;
  cert_changed: boolean;
  protocols_added?: string[];
  protocols_removed?: string[];
  vulns_added?: string[];
  vulns_removed?: string[];
}

export interface ScanDiff {
  host_id: number;
  from: ScanRef;
  to: ScanRef;
  overall_grade_changed: boolean;
  endpoints: EndpointDiff[];
}

export interface ScanEndpoint {
  id: number;
  ip_address: string;
  server_name?: string;
  grade: string;
  grade_trust_ignored?: string;
  has_warnings: boolean;
  is_exceptional: boolean;
  status_message?: string;
  cert_not_after?: string;
  progress: number;
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
