import { create } from "zustand";
import { persist } from "zustand/middleware";

import type { UISession } from "@/lib/types";

interface SessionState {
  apiBaseURL: string;
  selectedInvoiceID: string;
  session: UISession | null;
  setAPIBaseURL: (value: string) => void;
  setSelectedInvoiceID: (value: string) => void;
  setSession: (value: UISession | null) => void;
}

export const useSessionStore = create<SessionState>()(
  persist(
    (set) => ({
      apiBaseURL: "",
      selectedInvoiceID: "",
      session: null,
      setAPIBaseURL: (value) => set({ apiBaseURL: value.trim() }),
      setSelectedInvoiceID: (value) => set({ selectedInvoiceID: value.trim() }),
      setSession: (value) => set({ session: value }),
    }),
    {
      name: "billing-ops-ui-state-v2",
      partialize: (state) => ({
        apiBaseURL: state.apiBaseURL,
        selectedInvoiceID: state.selectedInvoiceID,
      }),
    }
  )
);
