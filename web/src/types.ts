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
