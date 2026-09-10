const BASE_URL = "http://localhost:8080";

export type Drug = {
  id: string;
  brand_name?: string;
  generic_name?: string;
  manufacturer?: string;
  product_ndc?: string;
  product_type?: string;
  route?: string;
  substance_name?: string;
};

export async function searchDrugs(query: string, limit = 20): Promise<Drug[]> {
  const res = await fetch(`${BASE_URL}/drugs?q=${encodeURIComponent(query)}&limit=${limit}`);

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error ?? `search failed (${res.status})`);
  }

  return res.json();
}
export async function deleteDrug(id: string): Promise<void> {
  const res = await fetch(`${BASE_URL}/drugs/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error ?? `delete failed (${res.status})`);
  }
}
export async function createDrug(drug: Drug): Promise<Drug> {
  const res = await fetch(`${BASE_URL}/drugs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(drug),
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error ?? `create failed (${res.status})`);
  }

  return res.json();
}