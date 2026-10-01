-- +migrate Up

CREATE TABLE authara.maintenance_leases (
	name varchar(64) PRIMARY KEY,
	owner_id uuid,
	lease_until timestamptz,
	generation bigint NOT NULL DEFAULT 0,
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT maintenance_lease_owner_expiry_pair CHECK (
		(owner_id IS NULL AND lease_until IS NULL)
		OR (owner_id IS NOT NULL AND lease_until IS NOT NULL)
	)
);

INSERT INTO authara.maintenance_leases (name)
VALUES ('cleanup');

INSERT INTO public.authara_schema_version (version)
VALUES (34)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 34;

DROP TABLE authara.maintenance_leases;
