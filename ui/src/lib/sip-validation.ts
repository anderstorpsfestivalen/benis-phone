import type { Definition } from "../generated/config";

export function validateSIPDefinition(doc: Definition): string[] {
  const errors: string[] = [];
  const fnNames = new Set((doc.fn ?? []).map((fn) => fn.name).filter(Boolean));
  const connectionIDs = new Set<string>();
  const ports = new Set<string>();
  const connections = doc.sip?.connection ?? [];
  if (connections.length === 0) errors.push("Add at least one SIP connection.");
  for (const [index, connection] of connections.entries()) {
    const label = connection.name || `Connection ${index + 1}`;
    if (
      !/^[A-Za-z0-9_-]{1,128}$/.test(connection.id) ||
      connectionIDs.has(connection.id)
    )
      errors.push(
        `${label}: connection ID must be present, unique, and use only letters, numbers, _ or -.`,
      );
    connectionIDs.add(connection.id);
    if (!["endpoint", "trunk"].includes(connection.kind))
      errors.push(`${label}: kind must be endpoint or trunk.`);
    if (!["registered", "inbound"].includes(connection.registration))
      errors.push(`${label}: mode must be registered or inbound.`);
    if (!["udp", "tcp", "ws"].includes(connection.transport || "udp"))
      errors.push(`${label}: transport must be udp, tcp, or ws.`);
    if (connection.local_port < 0 || connection.local_port > 65535)
      errors.push(`${label}: local port must be 0–65535.`);
    if (connection.local_port > 0) {
      const key = `${connection.transport || "udp"}:${connection.local_port}`;
      if (ports.has(key))
        errors.push(`${label}: ${key} is already used by another connection.`);
      ports.add(key);
    }
    if (
      connection.registration === "registered" &&
      (!connection.server || !connection.extension)
    ) {
      errors.push(
        `${label}: registered connections require server and extension.`,
      );
    }
    if (
      connection.registration === "inbound" &&
      (!connection.local_port || (connection.allowed_cidrs ?? []).length === 0)
    ) {
      errors.push(
        `${label}: inbound connections require an explicit port and at least one allowed CIDR.`,
      );
    }
    for (const cidr of connection.allowed_cidrs ?? []) {
      if (!validCIDR(cidr))
        errors.push(`${label}: ${cidr} is not a valid IPv4/IPv6 CIDR.`);
    }
    if (connection.kind === "endpoint") {
      if (!fnNames.has(connection.entrypoint))
        errors.push(`${label}: select a valid entrypoint.`);
      if ((connection.route ?? []).length)
        errors.push(
          `${label}: endpoint connections cannot contain trunk routes.`,
        );
      continue;
    }
    if ((connection.route ?? []).length === 0)
      errors.push(`${label}: add at least one trunk route.`);
    const routeIDs = new Set<string>();
    const numbers = new Set<string>();
    let catchAlls = 0;
    for (const route of connection.route ?? []) {
      if (!/^[A-Za-z0-9_-]{1,128}$/.test(route.id) || routeIDs.has(route.id))
        errors.push(
          `${label}: route IDs must be present, unique, and use only letters, numbers, _ or -.`,
        );
      routeIDs.add(route.id);
      if (!fnNames.has(route.entrypoint))
        errors.push(`${label}: every route needs a valid entrypoint.`);
      if (route.catch_all) {
        catchAlls++;
        if ((route.number ?? "").trim())
          errors.push(`${label}: a catch-all cannot also have a number.`);
      } else {
        const number = (route.number ?? "").trim().replace(/^\+/, "");
        if (!number || numbers.has(number))
          errors.push(
            `${label}: exact route numbers must be present and unique.`,
          );
        numbers.add(number);
      }
    }
    if (catchAlls > 1)
      errors.push(`${label}: only one catch-all route is allowed.`);
  }
  return [...new Set(errors)];
}

function validCIDR(value: string): boolean {
  const [address, prefix, extra] = value.trim().split("/");
  if (
    !address ||
    prefix === undefined ||
    extra !== undefined ||
    !/^\d+$/.test(prefix)
  )
    return false;
  const bits = Number(prefix);
  if (address.includes(":")) {
    if (bits < 0 || bits > 128 || !/^[0-9a-fA-F:]+$/.test(address))
      return false;
    try {
      return new URL(`http://[${address}]/`).hostname.includes(":");
    } catch {
      return false;
    }
  }
  const octets = address.split(".");
  return (
    bits >= 0 &&
    bits <= 32 &&
    octets.length === 4 &&
    octets.every((part) => /^\d{1,3}$/.test(part) && Number(part) <= 255)
  );
}
