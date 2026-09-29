-- Maps a product name to the GitHub repository an internal issue is filed in.
-- One row per product. Many products may share one repository.
-- No foreign key to product: staging cases are read from ServiceNow, while
-- this table stays in Postgres.
--
-- 0141–0150 were claimed by parallel RLS work (see 0151).

CREATE TABLE IF NOT EXISTS product_repo_mapping (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    VARCHAR(255) NOT NULL,
    updated_by    VARCHAR(255) NOT NULL,
    product_name  VARCHAR(255) NOT NULL,
    abbreviation  VARCHAR(64),
    owner         VARCHAR(255) NOT NULL,
    repository    VARCHAR(255) NOT NULL,
    github_label  VARCHAR(255) NOT NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT uq_product_repo_mapping_product_name UNIQUE (product_name),
    CONSTRAINT uq_product_repo_mapping_abbreviation UNIQUE (abbreviation)
);

CREATE INDEX IF NOT EXISTS idx_product_repo_mapping_active
    ON product_repo_mapping (is_active);

-- Seed from the internal product-to-repo sheet. product_name is the catalogue
-- name a case sends. abbreviation is set only where the case payload is known
-- to use a short code (wso2am, wso2am-analytics).
INSERT INTO product_repo_mapping (
    created_by, updated_by, product_name, abbreviation, owner, repository, github_label
) VALUES
    ('migration', 'migration', 'WSO2 Identity Platform (Asgardeo)', NULL, 'wso2-enterprise', 'wso2-iam-internal', 'Asgardeo'),
    ('migration', 'migration', 'WSO2 Identity Server', NULL, 'wso2-enterprise', 'wso2-iam-internal', 'IS'),
    ('migration', 'migration', 'WSO2 Identity Server Analytics', NULL, 'wso2-enterprise', 'wso2-iam-internal', 'IS-Analytics'),
    ('migration', 'migration', 'WSO2 Developer Platform (Choreo)', NULL, 'wso2-enterprise', 'choreo', 'Choreo'),
    ('migration', 'migration', 'Choreo-Analytics', NULL, 'wso2-enterprise', 'choreo', 'Choreo-Analytics'),
    ('migration', 'migration', 'Choreo-Connect', NULL, 'wso2-enterprise', 'choreo', 'Choreo-Connect'),
    ('migration', 'migration', 'WSO2 API Manager', 'wso2am', 'wso2-enterprise', 'wso2-apim-internal', 'APIM'),
    ('migration', 'migration', 'WSO2 API Manager Analytics', 'wso2am-analytics', 'wso2-enterprise', 'wso2-apim-internal', 'APIM-Analytics'),
    ('migration', 'migration', 'APAG', NULL, 'wso2-enterprise', 'wso2-apim-internal', 'APAG'),
    ('migration', 'migration', 'APK', NULL, 'wso2-enterprise', 'wso2-apim-internal', 'APK'),
    ('migration', 'migration', 'APG', NULL, 'wso2-enterprise', 'wso2-apim-internal', 'APG'),
    ('migration', 'migration', 'Bijira', NULL, 'wso2-enterprise', 'wso2-apim-internal', 'Bijira'),
    ('migration', 'migration', 'Microgateway', NULL, 'wso2-enterprise', 'wso2-apim-internal', 'Microgateway'),
    ('migration', 'migration', 'CSP', NULL, 'wso2-enterprise', 'digiops-cs', 'CSP'),
    ('migration', 'migration', 'Ballerina', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'Ballerina'),
    ('migration', 'migration', 'Devant', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'Devant'),
    ('migration', 'migration', 'EI', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'EI'),
    ('migration', 'migration', 'EI-Analytics', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'EI-Analytics'),
    ('migration', 'migration', 'ESB', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'ESB'),
    ('migration', 'migration', 'BI', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'BI'),
    ('migration', 'migration', 'ICP', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'ICP'),
    ('migration', 'migration', 'MI', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'MI'),
    ('migration', 'migration', 'SI', NULL, 'wso2-enterprise', 'wso2-integration-internal', 'SI'),
    ('migration', 'migration', 'OB-AM', NULL, 'wso2-enterprise', 'wso2-solutions-internal', 'OB-AM'),
    ('migration', 'migration', 'OB-BI', NULL, 'wso2-enterprise', 'wso2-solutions-internal', 'OB-BI'),
    ('migration', 'migration', 'OB-IAM', NULL, 'wso2-enterprise', 'wso2-solutions-internal', 'OB-IAM'),
    ('migration', 'migration', 'OB-KM', NULL, 'wso2-enterprise', 'wso2-solutions-internal', 'OB-KM')
ON CONFLICT (product_name) DO NOTHING;
