const BASE = "/controllers";
const GENERIC = `${BASE}/generic.png`;

const BY_MODEL: Record<string, string> = {
  DSE8610: `${BASE}/dse8610.png`,
  "DSE7320 MKII": `${BASE}/dse7320.png`,
  DSE7320: `${BASE}/dse7320.png`,
  DSE4520: `${BASE}/dse4520.png`,
  "DSE4520 MKII": `${BASE}/dse4520.png`,
  DSE6120: `${BASE}/dse6120.png`,
  "DEEP SEA 6120": `${BASE}/dse6120.png`,
  "DSE 6120": `${BASE}/dse6120.png`,
  INTELILITE9: `${BASE}/intelilite9.png`,
  "INTELILITE 9": `${BASE}/intelilite9.png`,
  "COMAP INTELILITE 9": `${BASE}/intelilite9.png`,
  INTELIMAINS150: `${BASE}/intelimains150.png`,
  "INTELIMAINS 150": `${BASE}/intelimains150.png`,
  "COMAP INTELIMAINS 150": `${BASE}/intelimains150.png`
};

function normalize(model: string) { return model.toUpperCase().replace(/\s+/g, " ").trim(); }

export function controllerImageSrc(model: string | null | undefined) {
  if (!model) return GENERIC;
  const key = normalize(model);
  if (BY_MODEL[key]) return BY_MODEL[key];
  const packed = key.replace(/[\s-]/g, "");
  if (BY_MODEL[packed]) return BY_MODEL[packed];
  if (key.includes("8610")) return BY_MODEL.DSE8610 ?? GENERIC;
  if (key.includes("7320")) return BY_MODEL.DSE7320 ?? GENERIC;
  if (key.includes("4520")) return BY_MODEL.DSE4520 ?? GENERIC;
  if (key.includes("6120")) return BY_MODEL.DSE6120 ?? GENERIC;
  return GENERIC;
}

export const CONTROLLER_IMAGE_FALLBACK = GENERIC;
