import { create } from "zustand";
import { persist } from "zustand/middleware";

interface SessionState {
  apiBaseURL: string;
  selectedInvoiceID: string;
  setAPIBaseURL: (value: string) => void;
  setSelectedInvoiceID: (value: string) => void;
}

export const useSessionStore = create<SessionState>()(
  persist(
    (set) => ({
      apiBaseURL: "",
      selectedInvoiceID: "",
      setAPIBaseURL: (value) => set({ apiBaseURL: value.trim() }),
      setSelectedInvoiceID: (value) => set({ selectedInvoiceID: value.trim() }),
    }),
    {
      name: "billing-ops-ui-state-v2",
    }
  )
);
