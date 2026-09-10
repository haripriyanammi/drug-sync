"use client";

import { useState } from "react";
import { searchDrugs, deleteDrug, createDrug, type Drug } from "@/lib/api";

const emptyForm: Drug = {
  id: "",
  brand_name: "",
  generic_name: "",
  manufacturer: "",
  route: "",
};

export default function Home() {
  const [query, setQuery] = useState("");
  const [drugs, setDrugs] = useState<Drug[]>([]);
  const [error, setError] = useState("");
  const [form, setForm] = useState<Drug>(emptyForm);

  async function handleSearch() {
    setError("");
    try {
      const results = await searchDrugs(query);
      setDrugs(results);
    } catch (e) {
      setError(e instanceof Error ? e.message : "something went wrong");
    }
  }

  async function handleDelete(id: string) {
    setError("");
    try {
      await deleteDrug(id);
      setDrugs(drugs.filter((d) => d.id !== id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "delete failed");
    }
  }

  async function handleCreate() {
    setError("");
    try {
      const created = await createDrug(form);
      setDrugs([created, ...drugs]);
      setForm(emptyForm);
    } catch (e) {
      setError(e instanceof Error ? e.message : "create failed");
    }
  }

  function updateForm(field: keyof Drug, value: string) {
    setForm({ ...form, [field]: value });
  }

  return (
    <main className="p-8 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Drug Search</h1>

      <div className="flex gap-2 mb-8">
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="tylenol"
          className="border px-3 py-2 rounded flex-1"
        />
        <button
          onClick={handleSearch}
          className="bg-blue-600 text-white px-4 py-2 rounded"
        >
          Search
        </button>
      </div>

      <div className="border rounded p-4 mb-8">
        <h2 className="font-semibold mb-3">Add a drug</h2>
        <div className="grid grid-cols-2 gap-2 mb-3">
          <input
            value={form.id}
            onChange={(e) => updateForm("id", e.target.value)}
            placeholder="id (required)"
            className="border px-3 py-2 rounded"
          />
          <input
            value={form.brand_name}
            onChange={(e) => updateForm("brand_name", e.target.value)}
            placeholder="brand name"
            className="border px-3 py-2 rounded"
          />
          <input
            value={form.generic_name}
            onChange={(e) => updateForm("generic_name", e.target.value)}
            placeholder="generic name"
            className="border px-3 py-2 rounded"
          />
          <input
            value={form.manufacturer}
            onChange={(e) => updateForm("manufacturer", e.target.value)}
            placeholder="manufacturer"
            className="border px-3 py-2 rounded"
          />
        </div>
        <button
          onClick={handleCreate}
          className="bg-green-600 text-white px-4 py-2 rounded"
        >
          Add
        </button>
      </div>

      {error && <p className="text-red-600 mb-4">{error}</p>}

      <ul className="space-y-3">
        {drugs.map((d) => (
          <li
            key={d.id}
            className="border rounded p-3 flex justify-between items-start"
          >
            <div>
              <p className="font-semibold">{d.brand_name}</p>
              <p className="text-sm text-gray-600">{d.generic_name}</p>
              <p className="text-sm text-gray-600">{d.manufacturer}</p>
            </div>
            <button
              onClick={() => handleDelete(d.id)}
              className="text-red-600 text-sm"
            >
              Delete
            </button>
          </li>
        ))}
      </ul>
    </main>
  );
} 