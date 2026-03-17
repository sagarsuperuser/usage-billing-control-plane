import { create } from "zustand";
import { persist } from "zustand/middleware";

import type { UISession } from "@/lib/types";

interface SessionState {
  selectedInvoiceID: string;
  session: UISession | null;
  setSelectedInvoiceID: (value: string) => void;
  setSession: (value: UISession | null) => void;
}

export const useSessionStore = create<SessionState>()(
  persist(
    (set) => ({
      selectedInvoiceID: "",
      session: null,
      setSelectedInvoiceID: (value) => set({ selectedInvoiceID: value.trim() }),
      setSession: (value) => set({ session: value }),
    }),
    {
      name: "billing-ops-ui-state-v3",
      partialize: (state) => ({
        selectedInvoiceID: state.selectedInvoiceID,
      }),
    }
  )
);
