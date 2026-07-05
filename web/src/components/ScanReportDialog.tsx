import { useEffect, useState } from "react";
import Modal from "./Modal";
import { api, ApiError } from "../lib/api";
import { gradeClasses, gradeLabel } from "../lib/grade";
import type { RawEndpoint, RawEndpointDetails, RawHost, Scan, ScanEndpoint } from "../types";

export default function ScanReportDialog({
  scanId,
  hostname,
  onClose,
}: {
  scanId: number;
  hostname: string;
  onClose: () => void;
}) {
  const [scan, setScan] = useState<Scan | null>(null);
  const [raw, setRaw] = useState<RawHost | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    (async () => {
      try {
        setScan(await api.getScan(scanId));
        // Error scans have no raw payload; ignore a 404 there.
        try {
          setRaw(await api.getScanRaw(scanId));
        } catch (e) {
          if (!(e instanceof ApiError) || e.status !== 404) throw e;
        }
      } catch (e) {
        setError(e instanceof Error ? e.message : "failed to load report");
      } finally {
        setLoading(false);
      }
    })();
  }, [scanId]);

  function downloadRaw() {
    if (!raw) return;
    const blob = new Blob([JSON.stringify(raw, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `scan-${scanId}-${hostname}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }

  const rawByIP = new Map<string, RawEndpoint>();
  (raw?.endpoints ?? []).forEach((e) => rawByIP.set(e.ipAddress, e));

  return (
    <Modal title={`Scan report — ${hostname}`} onClose={onClose} size="xl">
      {loading ? (
        <p className="text-sm text-slate-500">Loading…</p>
      ) : error ? (
        <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
      ) : !scan ? (
        <p className="text-sm text-slate-500">No report.</p>
      ) : (
        <div className="space-y-4">
          <Summary scan={scan} raw={raw} hostname={hostname} onDownload={raw ? downloadRaw : undefined} />

          {scan.status === "error" ? (
            <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
              Scan failed: {scan.error_message || "unknown error"}
            </div>
          ) : (scan.endpoints ?? []).length === 0 ? (
            <p className="text-sm text-slate-500">No endpoint results.</p>
          ) : (
            <div className="space-y-3">
              {scan.endpoints!.map((ep) => (
                <EndpointCard key={ep.id} ep={ep} raw={rawByIP.get(ep.ip_address)} certs={raw?.certs} />
              ))}
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}

function Summary({
  scan,
  raw,
  hostname,
  onDownload,
}: {
  scan: Scan;
  raw: RawHost | null;
  hostname: string;
  onDownload?: () => void;
}) {
  return (
    <div className="rounded-lg border border-slate-200 p-3 dark:border-slate-800">
      <div className="flex flex-wrap items-center gap-3">
        <span className={`rounded px-2 py-0.5 text-sm font-semibold ${gradeClasses(scan.overall_grade)}`}>
          {gradeLabel(scan.overall_grade)}
        </span>
        <span className="text-sm text-slate-500 dark:text-slate-400">status: {scan.status}</span>
        <span className="text-sm text-slate-500 dark:text-slate-400">trigger: {scan.trigger}</span>
        <a
          href={`https://www.ssllabs.com/ssltest/analyze.html?d=${encodeURIComponent(hostname)}`}
          target="_blank"
          rel="noreferrer"
          className="text-sm text-blue-600 underline dark:text-blue-400"
        >
          SSL Labs ↗
        </a>
        {onDownload && (
          <button
            type="button"
            onClick={onDownload}
            className="ml-auto rounded-md border border-slate-300 px-2.5 py-1 text-xs hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
          >
            Download raw JSON
          </button>
        )}
      </div>
      <dl className="mt-2 grid grid-cols-2 gap-x-4 gap-y-1 text-xs text-slate-500 dark:text-slate-400 sm:grid-cols-4">
        <Meta label="Engine" value={scan.engine_version || raw?.engineVersion} />
        <Meta label="Criteria" value={scan.criteria_version || raw?.criteriaVersion} />
        <Meta label="Started" value={fmt(scan.started_at)} />
        <Meta label="Completed" value={fmt(scan.completed_at ?? scan.created_at)} />
      </dl>
    </div>
  );
}

function EndpointCard({
  ep,
  raw,
  certs,
}: {
  ep: ScanEndpoint;
  raw?: RawEndpoint;
  certs?: RawHost["certs"];
}) {
  const vulns = vulnerabilities(raw?.details);
  const protocols = (raw?.details?.protocols ?? []).map((p) => `${p.name} ${p.version}`);
  const cert = leafCert(raw, certs);

  return (
    <div className="rounded-lg border border-slate-200 p-3 dark:border-slate-800">
      <div className="flex flex-wrap items-center gap-2">
        <span className={`rounded px-1.5 py-0.5 text-xs font-semibold ${gradeClasses(ep.grade)}`}>
          {gradeLabel(ep.grade)}
        </span>
        <span className="font-mono text-sm">{ep.ip_address}</span>
        {ep.server_name && <span className="text-xs text-slate-500 dark:text-slate-400">{ep.server_name}</span>}
        {ep.has_warnings && <Chip tone="amber">warnings</Chip>}
        {ep.is_exceptional && <Chip tone="green">exceptional</Chip>}
        {ep.grade_trust_ignored && ep.grade_trust_ignored !== ep.grade && (
          <span className="text-xs text-slate-500 dark:text-slate-400">
            (trust-ignored: {ep.grade_trust_ignored})
          </span>
        )}
      </div>
      {ep.status_message && ep.status_message !== "Ready" && (
        <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">{ep.status_message}</p>
      )}

      <dl className="mt-2 space-y-1 text-xs">
        {ep.cert_not_after && (
          <Row label="Certificate expires" value={new Date(ep.cert_not_after).toLocaleDateString()} />
        )}
        {cert?.subject && <Row label="Subject" value={cert.subject} mono />}
        {cert?.issuerSubject && <Row label="Issuer" value={cert.issuerSubject} mono />}
        {protocols.length > 0 && <Row label="Protocols" value={protocols.join(", ")} />}
        <Row
          label="Vulnerabilities"
          value={vulns.length > 0 ? vulns.join(", ") : "none detected"}
          tone={vulns.length > 0 ? "red" : "green"}
        />
      </dl>
    </div>
  );
}

function vulnerabilities(d?: RawEndpointDetails): string[] {
  if (!d) return [];
  const v: string[] = [];
  if (d.heartbleed) v.push("Heartbleed");
  if (d.poodle) v.push("POODLE");
  if ((d.poodleTls ?? 0) > 1) v.push("POODLE-TLS");
  if (d.freak) v.push("FREAK");
  if (d.logjam) v.push("Logjam");
  if (d.drownVulnerable) v.push("DROWN");
  if (d.vulnBeast) v.push("BEAST");
  return v;
}

function leafCert(ep?: RawEndpoint, certs?: RawHost["certs"]) {
  const id = ep?.details?.certChains?.[0]?.certIds?.[0];
  if (!id || !certs) return undefined;
  return certs.find((c) => c.id === id);
}

function Chip({ tone, children }: { tone: "amber" | "green" | "red"; children: React.ReactNode }) {
  const cls =
    tone === "amber"
      ? "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300"
      : tone === "red"
        ? "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300"
        : "bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300";
  return <span className={`rounded px-1.5 py-0.5 text-xs ${cls}`}>{children}</span>;
}

function Row({ label, value, mono, tone }: { label: string; value: string; mono?: boolean; tone?: "red" | "green" }) {
  const valCls = tone === "red" ? "text-red-600 dark:text-red-400" : tone === "green" ? "text-green-600 dark:text-green-400" : "";
  return (
    <div className="flex gap-2">
      <dt className="w-36 shrink-0 text-slate-500 dark:text-slate-400">{label}</dt>
      <dd className={`${mono ? "break-all font-mono" : ""} ${valCls}`}>{value}</dd>
    </div>
  );
}

function Meta({ label, value }: { label: string; value?: string }) {
  return (
    <div>
      <dt className="text-slate-400">{label}</dt>
      <dd className="text-slate-600 dark:text-slate-300">{value || "—"}</dd>
    </div>
  );
}

function fmt(ts?: string): string {
  return ts ? new Date(ts).toLocaleString() : "";
}
