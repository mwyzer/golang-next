-- Seed a default tenant/user so the API is usable before a full
-- authentication system exists. Fixed IDs so dev tooling can reference them.
INSERT INTO tenants (id, name)
VALUES ('00000000-0000-0000-0000-000000000001', 'Default Tenant')
ON CONFLICT (id) DO NOTHING;

INSERT INTO users (id, tenant_id, email, role)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    '00000000-0000-0000-0000-000000000001',
    'dev@example.com',
    'admin'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO document_types (id, field_schema, validation_rules, auto_process_threshold)
VALUES
    ('invoice', '{"vendor_name": "string", "total_amount": "number", "invoice_date": "date"}', '{}', 0.9),
    ('receipt', '{"merchant_name": "string", "total_amount": "number", "purchase_date": "date"}', '{}', 0.9),
    ('cv', '{"candidate_name": "string", "email": "string", "skills": "array"}', '{}', 0.85)
ON CONFLICT (id) DO NOTHING;
