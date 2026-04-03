DROP POLICY IF EXISTS p_invoice_line_items_tenant ON invoice_line_items;
DROP POLICY IF EXISTS p_invoices_tenant ON invoices;
DROP TABLE IF EXISTS invoice_line_items;
DROP TABLE IF EXISTS invoices;
