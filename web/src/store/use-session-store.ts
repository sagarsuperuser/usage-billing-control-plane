import { create } from "zustand";
import { persist } from "zustand/middleware";

interface SessionState {
  apiKey: string;
  apiBaseURL: string;
  selectedInvoiceID: string;
  setAPIKey: (value: string) => void;
  setAPIBaseURL: (value: string) => void;
  setSelectedInvoiceID: (value: string) => void;
}

export const useSessionStore = create<SessionState>()(
  persist(
    (set) => ({
      apiKey: "",
      apiBaseURL: "",
      selectedInvoiceID: "",
      setAPIKey: (value) => set({ apiKey: value.trim() }),
      setAPIBaseURL: (value) => set({ apiBaseURL: value.trim() }),
      setSelectedInvoiceID: (value) => set({ selectedInvoiceID: value.trim() }),
    }),
    {
      name: "billing-ops-session-v1",
    }
  )
);
