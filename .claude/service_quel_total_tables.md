# ai_workflow_sessions

create table public.ai_workflow_sessions (
  session_id character varying(255) not null,
  user_id character varying(255) not null,
  title character varying(500) null,
  category character varying(50) null,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  messages jsonb null,
  workflow_snapshot jsonb null,
  node_count integer null default 0,
  message_count integer null default 0,
  inventory jsonb null default '[]'::jsonb,
  constraint ai_workflow_sessions_pkey primary key (session_id)
) TABLESPACE pg_default;

create index IF not exists idx_user_created on public.ai_workflow_sessions using btree (user_id, created_at desc) TABLESPACE pg_default;

create index IF not exists idx_user_updated on public.ai_workflow_sessions using btree (user_id, updated_at desc) TABLESPACE pg_default;

create index IF not exists idx_ai_workflow_sessions_inventory on public.ai_workflow_sessions using gin (inventory) TABLESPACE pg_default;

create trigger trigger_update_ai_workflow_sessions_updated_at BEFORE
update on ai_workflow_sessions for EACH row
execute FUNCTION update_ai_workflow_sessions_updated_at ();


# case_studies

create table public.organization_plan (
  id uuid not null default gen_random_uuid (),
  name text not null,
  price bigint not null,
  price_id text not null,
  currency text not null,
  country text not null,
  billing_period text null default 'monthly'::text,
  description text null,
  active boolean null default true,
  created_at timestamp with time zone null default now(),
  adress character varying null default 'development'::character varying,
  plan_type text null default 'member'::text,
  environment text null,
  constraint organization_plan_pkey primary key (id),
  constraint organization_plan_environment_check check (
    (
      environment = any (array['development'::text, 'production'::text])
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_org_plan_country_env on public.organization_plan using btree (country, environment, active) TABLESPACE pg_default;

# contact_inquiries 

create table public.organization_plan (
  id uuid not null default gen_random_uuid (),
  name text not null,
  price bigint not null,
  price_id text not null,
  currency text not null,
  country text not null,
  billing_period text null default 'monthly'::text,
  description text null,
  active boolean null default true,
  created_at timestamp with time zone null default now(),
  adress character varying null default 'development'::character varying,
  plan_type text null default 'member'::text,
  environment text null,
  constraint organization_plan_pkey primary key (id),
  constraint organization_plan_environment_check check (
    (
      environment = any (array['development'::text, 'production'::text])
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_org_plan_country_env on public.organization_plan using btree (country, environment, active) TABLESPACE pg_default;

# custom_categories

create table public.organization_plan (
  id uuid not null default gen_random_uuid (),
  name text not null,
  price bigint not null,
  price_id text not null,
  currency text not null,
  country text not null,
  billing_period text null default 'monthly'::text,
  description text null,
  active boolean null default true,
  created_at timestamp with time zone null default now(),
  adress character varying null default 'development'::character varying,
  plan_type text null default 'member'::text,
  environment text null,
  constraint organization_plan_pkey primary key (id),
  constraint organization_plan_environment_check check (
    (
      environment = any (array['development'::text, 'production'::text])
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_org_plan_country_env on public.organization_plan using btree (country, environment, active) TABLESPACE pg_default;

# custom_fonts

create table public.organization_plan (
  id uuid not null default gen_random_uuid (),
  name text not null,
  price bigint not null,
  price_id text not null,
  currency text not null,
  country text not null,
  billing_period text null default 'monthly'::text,
  description text null,
  active boolean null default true,
  created_at timestamp with time zone null default now(),
  adress character varying null default 'development'::character varying,
  plan_type text null default 'member'::text,
  environment text null,
  constraint organization_plan_pkey primary key (id),
  constraint organization_plan_environment_check check (
    (
      environment = any (array['development'::text, 'production'::text])
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_org_plan_country_env on public.organization_plan using btree (country, environment, active) TABLESPACE pg_default;

# landing_page_contents

create table public.organization_plan (
  id uuid not null default gen_random_uuid (),
  name text not null,
  price bigint not null,
  price_id text not null,
  currency text not null,
  country text not null,
  billing_period text null default 'monthly'::text,
  description text null,
  active boolean null default true,
  created_at timestamp with time zone null default now(),
  adress character varying null default 'development'::character varying,
  plan_type text null default 'member'::text,
  environment text null,
  constraint organization_plan_pkey primary key (id),
  constraint organization_plan_environment_check check (
    (
      environment = any (array['development'::text, 'production'::text])
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_org_plan_country_env on public.organization_plan using btree (country, environment, active) TABLESPACE pg_default;

# landing_templates

create table public.organization_plan (
  id uuid not null default gen_random_uuid (),
  name text not null,
  price bigint not null,
  price_id text not null,
  currency text not null,
  country text not null,
  billing_period text null default 'monthly'::text,
  description text null,
  active boolean null default true,
  created_at timestamp with time zone null default now(),
  adress character varying null default 'development'::character varying,
  plan_type text null default 'member'::text,
  environment text null,
  constraint organization_plan_pkey primary key (id),
  constraint organization_plan_environment_check check (
    (
      environment = any (array['development'::text, 'production'::text])
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_org_plan_country_env on public.organization_plan using btree (country, environment, active) TABLESPACE pg_default;

# organization_invite_token

create table public.organization_plan (
  id uuid not null default gen_random_uuid (),
  name text not null,
  price bigint not null,
  price_id text not null,
  currency text not null,
  country text not null,
  billing_period text null default 'monthly'::text,
  description text null,
  active boolean null default true,
  created_at timestamp with time zone null default now(),
  adress character varying null default 'development'::character varying,
  plan_type text null default 'member'::text,
  environment text null,
  constraint organization_plan_pkey primary key (id),
  constraint organization_plan_environment_check check (
    (
      environment = any (array['development'::text, 'production'::text])
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_org_plan_country_env on public.organization_plan using btree (country, environment, active) TABLESPACE pg_default;

# organization_member_payment

create table public.organization_plan (
  id uuid not null default gen_random_uuid (),
  name text not null,
  price bigint not null,
  price_id text not null,
  currency text not null,
  country text not null,
  billing_period text null default 'monthly'::text,
  description text null,
  active boolean null default true,
  created_at timestamp with time zone null default now(),
  adress character varying null default 'development'::character varying,
  plan_type text null default 'member'::text,
  environment text null,
  constraint organization_plan_pkey primary key (id),
  constraint organization_plan_environment_check check (
    (
      environment = any (array['development'::text, 'production'::text])
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_org_plan_country_env on public.organization_plan using btree (country, environment, active) TABLESPACE pg_default;

# organization_plan 

create table public.organization_plan (
  id uuid not null default gen_random_uuid (),
  name text not null,
  price bigint not null,
  price_id text not null,
  currency text not null,
  country text not null,
  billing_period text null default 'monthly'::text,
  description text null,
  active boolean null default true,
  created_at timestamp with time zone null default now(),
  adress character varying null default 'development'::character varying,
  plan_type text null default 'member'::text,
  environment text null,
  constraint organization_plan_pkey primary key (id),
  constraint organization_plan_environment_check check (
    (
      environment = any (array['development'::text, 'production'::text])
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_org_plan_country_env on public.organization_plan using btree (country, environment, active) TABLESPACE pg_default;

# organization_subscription

create table public.organization_subscription_history (
  id uuid not null default gen_random_uuid (),
  org_id uuid not null,
  subscription_id uuid null,
  change_type text not null,
  previous_tier text null,
  previous_max_members integer null,
  previous_amount integer null,
  new_tier text null,
  new_max_members integer null,
  new_amount integer null,
  currency text null,
  proration_amount integer null,
  stripe_subscription_id text null,
  stripe_invoice_id text null,
  changed_by uuid null,
  metadata jsonb null default '{}'::jsonb,
  created_at timestamp with time zone not null default now(),
  constraint organization_subscription_history_pkey primary key (id),
  constraint organization_subscription_history_changed_by_fkey foreign KEY (changed_by) references quel_member (quel_member_id),
  constraint organization_subscription_history_subscription_id_fkey foreign KEY (subscription_id) references organization_subscription (id) on delete set null,
  constraint organization_subscription_history_org_id_fkey foreign KEY (org_id) references quel_organization (org_id) on delete CASCADE,
  constraint organization_subscription_history_change_type_check check (
    (
      change_type = any (
        array[
          'upgrade'::text,
          'downgrade'::text,
          'initial'::text,
          'renewal'::text,
          'cancellation'::text,
          'reactivation'::text
        ]
      )
    )
  ),
  constraint organization_subscription_history_currency_check check (
    (
      currency = any (array['krw'::text, 'jpy'::text, 'usd'::text])
    )
  ),
  constraint organization_subscription_history_previous_tier_check check (
    (
      previous_tier = any (
        array['tier1'::text, 'tier2'::text, 'tier3'::text]
      )
    )
  ),
  constraint organization_subscription_history_new_tier_check check (
    (
      new_tier = any (
        array['tier1'::text, 'tier2'::text, 'tier3'::text]
      )
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_org_sub_history_org_id on public.organization_subscription_history using btree (org_id) TABLESPACE pg_default;

create index IF not exists idx_org_sub_history_change_type on public.organization_subscription_history using btree (change_type) TABLESPACE pg_default;

create index IF not exists idx_org_sub_history_created_at on public.organization_subscription_history using btree (created_at desc) TABLESPACE pg_default;

create index IF not exists idx_org_sub_history_subscription_id on public.organization_subscription_history using btree (subscription_id) TABLESPACE pg_default;

# organization_subscription_history

create table public.organization_subscription_history (
  id uuid not null default gen_random_uuid (),
  org_id uuid not null,
  subscription_id uuid null,
  change_type text not null,
  previous_tier text null,
  previous_max_members integer null,
  previous_amount integer null,
  new_tier text null,
  new_max_members integer null,
  new_amount integer null,
  currency text null,
  proration_amount integer null,
  stripe_subscription_id text null,
  stripe_invoice_id text null,
  changed_by uuid null,
  metadata jsonb null default '{}'::jsonb,
  created_at timestamp with time zone not null default now(),
  constraint organization_subscription_history_pkey primary key (id),
  constraint organization_subscription_history_changed_by_fkey foreign KEY (changed_by) references quel_member (quel_member_id),
  constraint organization_subscription_history_subscription_id_fkey foreign KEY (subscription_id) references organization_subscription (id) on delete set null,
  constraint organization_subscription_history_org_id_fkey foreign KEY (org_id) references quel_organization (org_id) on delete CASCADE,
  constraint organization_subscription_history_change_type_check check (
    (
      change_type = any (
        array[
          'upgrade'::text,
          'downgrade'::text,
          'initial'::text,
          'renewal'::text,
          'cancellation'::text,
          'reactivation'::text
        ]
      )
    )
  ),
  constraint organization_subscription_history_currency_check check (
    (
      currency = any (array['krw'::text, 'jpy'::text, 'usd'::text])
    )
  ),
  constraint organization_subscription_history_previous_tier_check check (
    (
      previous_tier = any (
        array['tier1'::text, 'tier2'::text, 'tier3'::text]
      )
    )
  ),
  constraint organization_subscription_history_new_tier_check check (
    (
      new_tier = any (
        array['tier1'::text, 'tier2'::text, 'tier3'::text]
      )
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_org_sub_history_org_id on public.organization_subscription_history using btree (org_id) TABLESPACE pg_default;

create index IF not exists idx_org_sub_history_change_type on public.organization_subscription_history using btree (change_type) TABLESPACE pg_default;

create index IF not exists idx_org_sub_history_created_at on public.organization_subscription_history using btree (created_at desc) TABLESPACE pg_default;

create index IF not exists idx_org_sub_history_subscription_id on public.organization_subscription_history using btree (subscription_id) TABLESPACE pg_default;

# organization_workspace_payment

create table public.quel_commission_rates (
  rate_id uuid not null default gen_random_uuid (),
  partner_id uuid null,
  company_rate numeric(5, 2) null default 80.00,
  partner_rate numeric(5, 2) null default 20.00,
  effective_date timestamp with time zone not null,
  created_by uuid null,
  created_at timestamp with time zone null default now(),
  notes text null,
  constraint quel_commission_rates_pkey primary key (rate_id),
  constraint quel_commission_rates_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_partner on public.quel_commission_rates using btree (partner_id) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_date on public.quel_commission_rates using btree (effective_date) TABLESPACE pg_default;

# partner_settlements

create table public.quel_commission_rates (
  rate_id uuid not null default gen_random_uuid (),
  partner_id uuid null,
  company_rate numeric(5, 2) null default 80.00,
  partner_rate numeric(5, 2) null default 20.00,
  effective_date timestamp with time zone not null,
  created_by uuid null,
  created_at timestamp with time zone null default now(),
  notes text null,
  constraint quel_commission_rates_pkey primary key (rate_id),
  constraint quel_commission_rates_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_partner on public.quel_commission_rates using btree (partner_id) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_date on public.quel_commission_rates using btree (effective_date) TABLESPACE pg_default;

# partners 

create table public.quel_commission_rates (
  rate_id uuid not null default gen_random_uuid (),
  partner_id uuid null,
  company_rate numeric(5, 2) null default 80.00,
  partner_rate numeric(5, 2) null default 20.00,
  effective_date timestamp with time zone not null,
  created_by uuid null,
  created_at timestamp with time zone null default now(),
  notes text null,
  constraint quel_commission_rates_pkey primary key (rate_id),
  constraint quel_commission_rates_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_partner on public.quel_commission_rates using btree (partner_id) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_date on public.quel_commission_rates using btree (effective_date) TABLESPACE pg_default;

# payments

create table public.quel_commission_rates (
  rate_id uuid not null default gen_random_uuid (),
  partner_id uuid null,
  company_rate numeric(5, 2) null default 80.00,
  partner_rate numeric(5, 2) null default 20.00,
  effective_date timestamp with time zone not null,
  created_by uuid null,
  created_at timestamp with time zone null default now(),
  notes text null,
  constraint quel_commission_rates_pkey primary key (rate_id),
  constraint quel_commission_rates_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_partner on public.quel_commission_rates using btree (partner_id) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_date on public.quel_commission_rates using btree (effective_date) TABLESPACE pg_default;

# plans 

create table public.quel_commission_rates (
  rate_id uuid not null default gen_random_uuid (),
  partner_id uuid null,
  company_rate numeric(5, 2) null default 80.00,
  partner_rate numeric(5, 2) null default 20.00,
  effective_date timestamp with time zone not null,
  created_by uuid null,
  created_at timestamp with time zone null default now(),
  notes text null,
  constraint quel_commission_rates_pkey primary key (rate_id),
  constraint quel_commission_rates_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_partner on public.quel_commission_rates using btree (partner_id) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_date on public.quel_commission_rates using btree (effective_date) TABLESPACE pg_default;

# projects

create table public.quel_commission_rates (
  rate_id uuid not null default gen_random_uuid (),
  partner_id uuid null,
  company_rate numeric(5, 2) null default 80.00,
  partner_rate numeric(5, 2) null default 20.00,
  effective_date timestamp with time zone not null,
  created_by uuid null,
  created_at timestamp with time zone null default now(),
  notes text null,
  constraint quel_commission_rates_pkey primary key (rate_id),
  constraint quel_commission_rates_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_partner on public.quel_commission_rates using btree (partner_id) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_date on public.quel_commission_rates using btree (effective_date) TABLESPACE pg_default;

# quel_admin_users

create table public.quel_commission_rates (
  rate_id uuid not null default gen_random_uuid (),
  partner_id uuid null,
  company_rate numeric(5, 2) null default 80.00,
  partner_rate numeric(5, 2) null default 20.00,
  effective_date timestamp with time zone not null,
  created_by uuid null,
  created_at timestamp with time zone null default now(),
  notes text null,
  constraint quel_commission_rates_pkey primary key (rate_id),
  constraint quel_commission_rates_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_partner on public.quel_commission_rates using btree (partner_id) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_date on public.quel_commission_rates using btree (effective_date) TABLESPACE pg_default;

# quel_attach

create table public.quel_commission_rates (
  rate_id uuid not null default gen_random_uuid (),
  partner_id uuid null,
  company_rate numeric(5, 2) null default 80.00,
  partner_rate numeric(5, 2) null default 20.00,
  effective_date timestamp with time zone not null,
  created_by uuid null,
  created_at timestamp with time zone null default now(),
  notes text null,
  constraint quel_commission_rates_pkey primary key (rate_id),
  constraint quel_commission_rates_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_partner on public.quel_commission_rates using btree (partner_id) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_date on public.quel_commission_rates using btree (effective_date) TABLESPACE pg_default;

# quel_commission_rates

create table public.quel_commission_rates (
  rate_id uuid not null default gen_random_uuid (),
  partner_id uuid null,
  company_rate numeric(5, 2) null default 80.00,
  partner_rate numeric(5, 2) null default 20.00,
  effective_date timestamp with time zone not null,
  created_by uuid null,
  created_at timestamp with time zone null default now(),
  notes text null,
  constraint quel_commission_rates_pkey primary key (rate_id),
  constraint quel_commission_rates_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_partner on public.quel_commission_rates using btree (partner_id) TABLESPACE pg_default;

create index IF not exists idx_commission_rates_date on public.quel_commission_rates using btree (effective_date) TABLESPACE pg_default;

# quel_credits

create table public.quel_credits (
  id uuid not null default gen_random_uuid (),
  user_id uuid not null,
  transaction_type character varying(20) not null,
  amount integer not null,
  balance_after integer not null,
  description text null,
  attach_idx bigint null,
  created_at timestamp with time zone not null default now(),
  production_idx uuid null,
  org_id uuid null,
  used_by_member_id uuid null,
  api_provider character varying null,
  updated_at timestamp with time zone null default now(),
  constraint quel_credits_pkey primary key (id),
  constraint quel_credits_attach_idx_fkey foreign KEY (attach_idx) references quel_attach (attach_id),
  constraint quel_credits_org_id_fkey foreign KEY (org_id) references quel_organization (org_id) on delete CASCADE,
  constraint quel_credits_production_idx_fkey foreign KEY (production_idx) references quel_production_photo (production_id),
  constraint quel_credits_used_by_member_id_fkey foreign KEY (used_by_member_id) references quel_member (quel_member_id),
  constraint quel_credits_user_id_fkey foreign KEY (user_id) references quel_member (quel_member_id)
) TABLESPACE pg_default;

create index IF not exists idx_quel_credits_created_at on public.quel_credits using btree (created_at) TABLESPACE pg_default;

create index IF not exists idx_quel_credits_user_id on public.quel_credits using btree (user_id) TABLESPACE pg_default;


# quel_member

create table public.quel_partner_payouts (
  payout_id uuid not null default gen_random_uuid (),
  partner_id uuid not null,
  amount bigint not null,
  stripe_payout_id character varying(255) null,
  status character varying(50) null default 'pending'::character varying,
  requested_at timestamp with time zone null default now(),
  completed_at timestamp with time zone null,
  failure_reason text null,
  notes text null,
  currency character varying(3) not null default 'usd'::character varying,
  stripe_account_id character varying(255) null,
  payout_month integer null,
  payout_year integer null,
  period_start_date date null,
  period_end_date date null,
  processed_at timestamp with time zone null,
  failed_at timestamp with time zone null,
  error_code character varying(100) null,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  constraint quel_partner_payouts_pkey primary key (payout_id),
  constraint quel_partner_payouts_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_partner_payouts_partner on public.quel_partner_payouts using btree (partner_id) TABLESPACE pg_default;

# quel_member_coupons

create table public.quel_partner_payouts (
  payout_id uuid not null default gen_random_uuid (),
  partner_id uuid not null,
  amount bigint not null,
  stripe_payout_id character varying(255) null,
  status character varying(50) null default 'pending'::character varying,
  requested_at timestamp with time zone null default now(),
  completed_at timestamp with time zone null,
  failure_reason text null,
  notes text null,
  currency character varying(3) not null default 'usd'::character varying,
  stripe_account_id character varying(255) null,
  payout_month integer null,
  payout_year integer null,
  period_start_date date null,
  period_end_date date null,
  processed_at timestamp with time zone null,
  failed_at timestamp with time zone null,
  error_code character varying(100) null,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  constraint quel_partner_payouts_pkey primary key (payout_id),
  constraint quel_partner_payouts_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_partner_payouts_partner on public.quel_partner_payouts using btree (partner_id) TABLESPACE pg_default;


# quel_organization

create table public.quel_partner_payouts (
  payout_id uuid not null default gen_random_uuid (),
  partner_id uuid not null,
  amount bigint not null,
  stripe_payout_id character varying(255) null,
  status character varying(50) null default 'pending'::character varying,
  requested_at timestamp with time zone null default now(),
  completed_at timestamp with time zone null,
  failure_reason text null,
  notes text null,
  currency character varying(3) not null default 'usd'::character varying,
  stripe_account_id character varying(255) null,
  payout_month integer null,
  payout_year integer null,
  period_start_date date null,
  period_end_date date null,
  processed_at timestamp with time zone null,
  failed_at timestamp with time zone null,
  error_code character varying(100) null,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  constraint quel_partner_payouts_pkey primary key (payout_id),
  constraint quel_partner_payouts_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_partner_payouts_partner on public.quel_partner_payouts using btree (partner_id) TABLESPACE pg_default;

# quel_organization_member

create table public.quel_partner_payouts (
  payout_id uuid not null default gen_random_uuid (),
  partner_id uuid not null,
  amount bigint not null,
  stripe_payout_id character varying(255) null,
  status character varying(50) null default 'pending'::character varying,
  requested_at timestamp with time zone null default now(),
  completed_at timestamp with time zone null,
  failure_reason text null,
  notes text null,
  currency character varying(3) not null default 'usd'::character varying,
  stripe_account_id character varying(255) null,
  payout_month integer null,
  payout_year integer null,
  period_start_date date null,
  period_end_date date null,
  processed_at timestamp with time zone null,
  failed_at timestamp with time zone null,
  error_code character varying(100) null,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  constraint quel_partner_payouts_pkey primary key (payout_id),
  constraint quel_partner_payouts_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_partner_payouts_partner on public.quel_partner_payouts using btree (partner_id) TABLESPACE pg_default;


# quel_organization_workspace

create table public.quel_partner_payouts (
  payout_id uuid not null default gen_random_uuid (),
  partner_id uuid not null,
  amount bigint not null,
  stripe_payout_id character varying(255) null,
  status character varying(50) null default 'pending'::character varying,
  requested_at timestamp with time zone null default now(),
  completed_at timestamp with time zone null,
  failure_reason text null,
  notes text null,
  currency character varying(3) not null default 'usd'::character varying,
  stripe_account_id character varying(255) null,
  payout_month integer null,
  payout_year integer null,
  period_start_date date null,
  period_end_date date null,
  processed_at timestamp with time zone null,
  failed_at timestamp with time zone null,
  error_code character varying(100) null,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  constraint quel_partner_payouts_pkey primary key (payout_id),
  constraint quel_partner_payouts_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_partner_payouts_partner on public.quel_partner_payouts using btree (partner_id) TABLESPACE pg_default;

# quel_partner_customers

create table public.quel_partner_payouts (
  payout_id uuid not null default gen_random_uuid (),
  partner_id uuid not null,
  amount bigint not null,
  stripe_payout_id character varying(255) null,
  status character varying(50) null default 'pending'::character varying,
  requested_at timestamp with time zone null default now(),
  completed_at timestamp with time zone null,
  failure_reason text null,
  notes text null,
  currency character varying(3) not null default 'usd'::character varying,
  stripe_account_id character varying(255) null,
  payout_month integer null,
  payout_year integer null,
  period_start_date date null,
  period_end_date date null,
  processed_at timestamp with time zone null,
  failed_at timestamp with time zone null,
  error_code character varying(100) null,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  constraint quel_partner_payouts_pkey primary key (payout_id),
  constraint quel_partner_payouts_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_partner_payouts_partner on public.quel_partner_payouts using btree (partner_id) TABLESPACE pg_default;


# quel_partner_payouts

create table public.quel_partner_payouts (
  payout_id uuid not null default gen_random_uuid (),
  partner_id uuid not null,
  amount bigint not null,
  stripe_payout_id character varying(255) null,
  status character varying(50) null default 'pending'::character varying,
  requested_at timestamp with time zone null default now(),
  completed_at timestamp with time zone null,
  failure_reason text null,
  notes text null,
  currency character varying(3) not null default 'usd'::character varying,
  stripe_account_id character varying(255) null,
  payout_month integer null,
  payout_year integer null,
  period_start_date date null,
  period_end_date date null,
  processed_at timestamp with time zone null,
  failed_at timestamp with time zone null,
  error_code character varying(100) null,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  constraint quel_partner_payouts_pkey primary key (payout_id),
  constraint quel_partner_payouts_partner_id_fkey foreign KEY (partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_partner_payouts_partner on public.quel_partner_payouts using btree (partner_id) TABLESPACE pg_default;

# quel_partners

create table public.quel_partners (
  partner_id uuid not null default gen_random_uuid (),
  partner_email character varying(255) not null,
  partner_name character varying(255) not null,
  partner_company character varying(255) null,
  partner_phone character varying(50) null,
  partner_status character varying(50) null default 'pending'::character varying,
  credit_code character varying(50) not null,
  stripe_account_id character varying(255) null,
  stripe_onboarding_completed boolean null default false,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  referrer_partner_id uuid null,
  partner_level integer null default 1,
  commission_rate numeric(5, 2) null default 0.00,
  stripe_dashboard_url text null,
  stripe_final_onboarding_completed boolean null default false,
  our_company boolean null default false,
  constraint quel_partners_pkey primary key (partner_id),
  constraint quel_partners_credit_code_key unique (credit_code),
  constraint quel_partners_partner_email_key unique (partner_email),
  constraint quel_partners_referrer_partner_id_fkey foreign KEY (referrer_partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_partners_credit_code on public.quel_partners using btree (credit_code) TABLESPACE pg_default;

create index IF not exists idx_partners_stripe_account on public.quel_partners using btree (stripe_account_id) TABLESPACE pg_default;


# quel_partners_referral_code

create table public.quel_production_jobs (
  job_id uuid not null default gen_random_uuid (),
  production_id uuid not null,
  job_type character varying(50) not null,
  stage_index integer null,
  stage_name character varying(100) null,
  batch_index integer null,
  job_status public.job_status_enum null default 'pending'::job_status_enum,
  total_images integer not null,
  completed_images integer null default 0,
  failed_images integer null default 0,
  job_input_data jsonb not null,
  generated_attach_ids jsonb null default '[]'::jsonb,
  error_message text null,
  retry_count integer null default 0,
  created_at timestamp with time zone null default now(),
  started_at timestamp with time zone null,
  completed_at timestamp with time zone null,
  updated_at timestamp with time zone null default now(),
  quel_member_id uuid null,
  estimated_credits integer null default 0,
  remaining_credits numeric(10, 2) null default 0,
  quel_production_path character varying(50) null,
  org_id uuid null,
  generated_urls text[] null,
  constraint quel_production_jobs_pkey primary key (job_id),
  constraint quel_production_jobs_org_id_fkey foreign KEY (org_id) references quel_organization (org_id) on delete CASCADE,
  constraint quel_production_jobs_production_id_fkey foreign KEY (production_id) references quel_production_photo (production_id) on delete CASCADE,
  constraint quel_production_jobs_quel_member_id_fkey foreign KEY (quel_member_id) references quel_member (quel_member_id),
  constraint quel_production_jobs_job_status_check check (
    (
      job_status = any (
        array[
          'pending'::job_status_enum,
          'processing'::job_status_enum,
          'completed'::job_status_enum,
          'failed'::job_status_enum,
          'cancelled'::job_status_enum,
          'user_cancelled'::job_status_enum
        ]
      )
    )
  ),
  constraint quel_production_jobs_job_type_check check (
    (
      (job_type)::text = any (
        (
          array[
            'single_batch'::character varying,
            'pipeline_stage'::character varying,
            'simple_general'::character varying,
            'simple_portrait'::character varying
          ]
        )::text[]
      )
    )
  ),
  constraint quel_production_jobs_check check (
    (
      (
        (
          (job_type)::text = any (
            (
              array[
                'single_batch'::character varying,
                'simple_general'::character varying,
                'simple_portrait'::character varying
              ]
            )::text[]
          )
        )
        and (batch_index is not null)
        and (stage_index is null)
      )
      or (
        ((job_type)::text = 'pipeline_stage'::text)
        and (stage_index is not null)
        and (batch_index is null)
      )
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_quel_production_jobs_path on public.quel_production_jobs using btree (quel_production_path) TABLESPACE pg_default;

create index IF not exists idx_quel_production_jobs_org_id on public.quel_production_jobs using btree (org_id) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_created on public.quel_production_jobs using btree (created_at desc) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_production_id on public.quel_production_jobs using btree (production_id) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_status on public.quel_production_jobs using btree (job_status) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_type_status on public.quel_production_jobs using btree (job_type, job_status) TABLESPACE pg_default;

create index IF not exists idx_jobs_member_status_created on public.quel_production_jobs using btree (quel_member_id, job_status, created_at desc) TABLESPACE pg_default;

create trigger update_quel_production_jobs_updated_at BEFORE
update on quel_production_jobs for EACH row
execute FUNCTION update_updated_at_column ();


# quel_personal_workspace

create table public.quel_production_jobs (
  job_id uuid not null default gen_random_uuid (),
  production_id uuid not null,
  job_type character varying(50) not null,
  stage_index integer null,
  stage_name character varying(100) null,
  batch_index integer null,
  job_status public.job_status_enum null default 'pending'::job_status_enum,
  total_images integer not null,
  completed_images integer null default 0,
  failed_images integer null default 0,
  job_input_data jsonb not null,
  generated_attach_ids jsonb null default '[]'::jsonb,
  error_message text null,
  retry_count integer null default 0,
  created_at timestamp with time zone null default now(),
  started_at timestamp with time zone null,
  completed_at timestamp with time zone null,
  updated_at timestamp with time zone null default now(),
  quel_member_id uuid null,
  estimated_credits integer null default 0,
  remaining_credits numeric(10, 2) null default 0,
  quel_production_path character varying(50) null,
  org_id uuid null,
  generated_urls text[] null,
  constraint quel_production_jobs_pkey primary key (job_id),
  constraint quel_production_jobs_org_id_fkey foreign KEY (org_id) references quel_organization (org_id) on delete CASCADE,
  constraint quel_production_jobs_production_id_fkey foreign KEY (production_id) references quel_production_photo (production_id) on delete CASCADE,
  constraint quel_production_jobs_quel_member_id_fkey foreign KEY (quel_member_id) references quel_member (quel_member_id),
  constraint quel_production_jobs_job_status_check check (
    (
      job_status = any (
        array[
          'pending'::job_status_enum,
          'processing'::job_status_enum,
          'completed'::job_status_enum,
          'failed'::job_status_enum,
          'cancelled'::job_status_enum,
          'user_cancelled'::job_status_enum
        ]
      )
    )
  ),
  constraint quel_production_jobs_job_type_check check (
    (
      (job_type)::text = any (
        (
          array[
            'single_batch'::character varying,
            'pipeline_stage'::character varying,
            'simple_general'::character varying,
            'simple_portrait'::character varying
          ]
        )::text[]
      )
    )
  ),
  constraint quel_production_jobs_check check (
    (
      (
        (
          (job_type)::text = any (
            (
              array[
                'single_batch'::character varying,
                'simple_general'::character varying,
                'simple_portrait'::character varying
              ]
            )::text[]
          )
        )
        and (batch_index is not null)
        and (stage_index is null)
      )
      or (
        ((job_type)::text = 'pipeline_stage'::text)
        and (stage_index is not null)
        and (batch_index is null)
      )
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_quel_production_jobs_path on public.quel_production_jobs using btree (quel_production_path) TABLESPACE pg_default;

create index IF not exists idx_quel_production_jobs_org_id on public.quel_production_jobs using btree (org_id) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_created on public.quel_production_jobs using btree (created_at desc) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_production_id on public.quel_production_jobs using btree (production_id) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_status on public.quel_production_jobs using btree (job_status) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_type_status on public.quel_production_jobs using btree (job_type, job_status) TABLESPACE pg_default;

create index IF not exists idx_jobs_member_status_created on public.quel_production_jobs using btree (quel_member_id, job_status, created_at desc) TABLESPACE pg_default;

create trigger update_quel_production_jobs_updated_at BEFORE
update on quel_production_jobs for EACH row
execute FUNCTION update_updated_at_column ();

# quel_production_jobs

create table public.quel_production_jobs (
  job_id uuid not null default gen_random_uuid (),
  production_id uuid not null,
  job_type character varying(50) not null,
  stage_index integer null,
  stage_name character varying(100) null,
  batch_index integer null,
  job_status public.job_status_enum null default 'pending'::job_status_enum,
  total_images integer not null,
  completed_images integer null default 0,
  failed_images integer null default 0,
  job_input_data jsonb not null,
  generated_attach_ids jsonb null default '[]'::jsonb,
  error_message text null,
  retry_count integer null default 0,
  created_at timestamp with time zone null default now(),
  started_at timestamp with time zone null,
  completed_at timestamp with time zone null,
  updated_at timestamp with time zone null default now(),
  quel_member_id uuid null,
  estimated_credits integer null default 0,
  remaining_credits numeric(10, 2) null default 0,
  quel_production_path character varying(50) null,
  org_id uuid null,
  generated_urls text[] null,
  constraint quel_production_jobs_pkey primary key (job_id),
  constraint quel_production_jobs_org_id_fkey foreign KEY (org_id) references quel_organization (org_id) on delete CASCADE,
  constraint quel_production_jobs_production_id_fkey foreign KEY (production_id) references quel_production_photo (production_id) on delete CASCADE,
  constraint quel_production_jobs_quel_member_id_fkey foreign KEY (quel_member_id) references quel_member (quel_member_id),
  constraint quel_production_jobs_job_status_check check (
    (
      job_status = any (
        array[
          'pending'::job_status_enum,
          'processing'::job_status_enum,
          'completed'::job_status_enum,
          'failed'::job_status_enum,
          'cancelled'::job_status_enum,
          'user_cancelled'::job_status_enum
        ]
      )
    )
  ),
  constraint quel_production_jobs_job_type_check check (
    (
      (job_type)::text = any (
        (
          array[
            'single_batch'::character varying,
            'pipeline_stage'::character varying,
            'simple_general'::character varying,
            'simple_portrait'::character varying
          ]
        )::text[]
      )
    )
  ),
  constraint quel_production_jobs_check check (
    (
      (
        (
          (job_type)::text = any (
            (
              array[
                'single_batch'::character varying,
                'simple_general'::character varying,
                'simple_portrait'::character varying
              ]
            )::text[]
          )
        )
        and (batch_index is not null)
        and (stage_index is null)
      )
      or (
        ((job_type)::text = 'pipeline_stage'::text)
        and (stage_index is not null)
        and (batch_index is null)
      )
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_quel_production_jobs_path on public.quel_production_jobs using btree (quel_production_path) TABLESPACE pg_default;

create index IF not exists idx_quel_production_jobs_org_id on public.quel_production_jobs using btree (org_id) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_created on public.quel_production_jobs using btree (created_at desc) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_production_id on public.quel_production_jobs using btree (production_id) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_status on public.quel_production_jobs using btree (job_status) TABLESPACE pg_default;

create index IF not exists idx_production_jobs_type_status on public.quel_production_jobs using btree (job_type, job_status) TABLESPACE pg_default;

create index IF not exists idx_jobs_member_status_created on public.quel_production_jobs using btree (quel_member_id, job_status, created_at desc) TABLESPACE pg_default;

create trigger update_quel_production_jobs_updated_at BEFORE
update on quel_production_jobs for EACH row
execute FUNCTION update_updated_at_column ();


# quel_production_photo

create table public.quel_service_referral_code (
  service_code_id uuid not null default gen_random_uuid (),
  tier2_partner_id uuid not null,
  service_code character varying(50) not null,
  total_customers integer null default 0,
  total_revenue numeric(12, 2) null default 0.00,
  is_active boolean null default true,
  created_at timestamp with time zone null default now(),
  constraint quel_service_referral_code_pkey primary key (service_code_id),
  constraint quel_service_referral_code_service_code_key unique (service_code),
  constraint quel_service_referral_code_tier2_partner_id_fkey foreign KEY (tier2_partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_service_code_tier2 on public.quel_service_referral_code using btree (tier2_partner_id) TABLESPACE pg_default;

create index IF not exists idx_service_code_code on public.quel_service_referral_code using btree (service_code) TABLESPACE pg_default;

create index IF not exists idx_service_code_active on public.quel_service_referral_code using btree (is_active) TABLESPACE pg_default;




# quel_service_referral_code

create table public.quel_service_referral_code (
  service_code_id uuid not null default gen_random_uuid (),
  tier2_partner_id uuid not null,
  service_code character varying(50) not null,
  total_customers integer null default 0,
  total_revenue numeric(12, 2) null default 0.00,
  is_active boolean null default true,
  created_at timestamp with time zone null default now(),
  constraint quel_service_referral_code_pkey primary key (service_code_id),
  constraint quel_service_referral_code_service_code_key unique (service_code),
  constraint quel_service_referral_code_tier2_partner_id_fkey foreign KEY (tier2_partner_id) references quel_partners (partner_id)
) TABLESPACE pg_default;

create index IF not exists idx_service_code_tier2 on public.quel_service_referral_code using btree (tier2_partner_id) TABLESPACE pg_default;

create index IF not exists idx_service_code_code on public.quel_service_referral_code using btree (service_code) TABLESPACE pg_default;

create index IF not exists idx_service_code_active on public.quel_service_referral_code using btree (is_active) TABLESPACE pg_default;



# settings

create table public.settings (
  id uuid not null default extensions.uuid_generate_v4 (),
  key text not null,
  value text null,
  created_at timestamp with time zone null default timezone ('utc'::text, now()),
  updated_at timestamp with time zone null default timezone ('utc'::text, now()),
  constraint settings_pkey primary key (id),
  constraint settings_key_key unique (key)
) TABLESPACE pg_default;

create trigger update_settings_updated_at BEFORE
update on settings for EACH row
execute FUNCTION update_updated_at_column ();



# studio_management

create table public.studio_management (
  id uuid not null default gen_random_uuid (),
  studio_type character varying(50) not null,
  category_id character varying(100) not null,
  name character varying(255) not null,
  name_ko character varying(255) null,
  name_ja character varying(255) null,
  name_zh character varying(255) null,
  name_vi character varying(255) null,
  name_th character varying(255) null,
  description text null,
  description_ko text null,
  description_ja text null,
  description_zh text null,
  description_vi text null,
  description_th text null,
  color character varying(20) null default '#EA401E'::character varying,
  thumbnail_url text null,
  icon character varying(50) null,
  display_order integer null default 0,
  is_active boolean null default true,
  is_featured boolean null default false,
  metadata jsonb null default '{}'::jsonb,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  constraint studio_management_pkey primary key (id),
  constraint studio_management_studio_type_category_id_key unique (studio_type, category_id)
) TABLESPACE pg_default;

create index IF not exists idx_studio_management_studio_type on public.studio_management using btree (studio_type) TABLESPACE pg_default;

create index IF not exists idx_studio_management_category_id on public.studio_management using btree (category_id) TABLESPACE pg_default;

create index IF not exists idx_studio_management_is_active on public.studio_management using btree (is_active) TABLESPACE pg_default;

create index IF not exists idx_studio_management_display_order on public.studio_management using btree (display_order) TABLESPACE pg_default;

create trigger trigger_studio_management_updated_at BEFORE
update on studio_management for EACH row
execute FUNCTION update_studio_management_updated_at ();


# workflow_templates

create table public.workflow_templates (
  id uuid not null default gen_random_uuid (),
  name character varying(255) not null,
  description text null,
  studio character varying(50) not null default 'visual'::character varying,
  category character varying(50) not null,
  chain_count integer null default 1,
  nodes jsonb not null default '[]'::jsonb,
  edges jsonb not null default '[]'::jsonb,
  viewport jsonb null,
  thumbnail_url text null,
  is_active boolean null default true,
  is_featured boolean null default false,
  display_order integer null default 0,
  created_by uuid null,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  comments jsonb null,
  constraint workflow_templates_pkey primary key (id),
  constraint workflow_templates_created_by_fkey foreign KEY (created_by) references auth.users (id)
) TABLESPACE pg_default;

create index IF not exists idx_workflow_templates_studio on public.workflow_templates using btree (studio) TABLESPACE pg_default;

create index IF not exists idx_workflow_templates_category on public.workflow_templates using btree (category) TABLESPACE pg_default;

create index IF not exists idx_workflow_templates_active on public.workflow_templates using btree (is_active) TABLESPACE pg_default;

# workflows

create table public.workflows (
  id uuid not null default gen_random_uuid (),
  user_id text not null,
  name text not null,
  workflow_data jsonb not null,
  category text null,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  constraint workflows_pkey primary key (id)
) TABLESPACE pg_default;

create index IF not exists idx_workflows_user_id on public.workflows using btree (user_id) TABLESPACE pg_default;

create index IF not exists idx_workflows_updated_at on public.workflows using btree (updated_at desc) TABLESPACE pg_default;

create trigger workflows_updated_at_trigger BEFORE
update on workflows for EACH row
execute FUNCTION update_workflows_updated_at ();


# quel_organization (ALTER - 엔터프라이즈 컬럼 추가)

ALTER TABLE quel_organization
ADD COLUMN is_enterprise BOOLEAN DEFAULT FALSE,
ADD COLUMN enterprise_tier VARCHAR(50);

create index IF not exists idx_quel_organization_is_enterprise on public.quel_organization using btree (is_enterprise) TABLESPACE pg_default;


# enterprise_company_info (신규 테이블)

create table public.enterprise_company_info (
  id uuid not null default gen_random_uuid (),
  org_id uuid not null,

  -- 회사 정보
  company_name character varying(255) not null,
  business_registration_number character varying(20) null,
  representative_name character varying(100) null,

  -- 담당자 정보
  contact_name character varying(100) null,
  contact_email character varying(255) null,
  contact_phone character varying(20) null,

  -- 주소
  address character varying(500) null,

  -- 계약 정보
  contract_start_date date null,
  contract_end_date date null,
  contract_terms text null,

  -- 메타
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  created_by uuid null,

  constraint enterprise_company_info_pkey primary key (id),
  constraint enterprise_company_info_org_id_fkey foreign KEY (org_id) references quel_organization (org_id) on delete CASCADE,
  constraint enterprise_company_info_org_id_key unique (org_id)
) TABLESPACE pg_default;

create index IF not exists idx_enterprise_company_info_org_id on public.enterprise_company_info using btree (org_id) TABLESPACE pg_default;

create index IF not exists idx_enterprise_company_info_brn on public.enterprise_company_info using btree (business_registration_number) TABLESPACE pg_default;


# enterprise_transactions (신규 테이블)

create table public.enterprise_transactions (
  id uuid not null default gen_random_uuid (),
  org_id uuid not null,

  -- 거래 정보
  transaction_type character varying(50) not null,
  amount numeric(12, 2) not null,
  credits_amount integer null,

  -- 결제 정보
  payment_method character varying(50) null,
  payment_reference character varying(255) null,
  invoice_number character varying(100) null,

  -- 상태
  status character varying(50) not null default 'pending',
  confirmed_at timestamp with time zone null,
  confirmed_by uuid null,

  -- 메모
  admin_memo text null,

  -- 메타
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  created_by uuid null,

  constraint enterprise_transactions_pkey primary key (id),
  constraint enterprise_transactions_org_id_fkey foreign KEY (org_id) references quel_organization (org_id) on delete CASCADE,
  constraint enterprise_transactions_transaction_type_check check (
    (
      (transaction_type)::text = any (
        array['credit_charge'::text, 'refund'::text]
      )
    )
  ),
  constraint enterprise_transactions_status_check check (
    (
      (status)::text = any (
        array['pending'::text, 'confirmed'::text, 'cancelled'::text]
      )
    )
  )
) TABLESPACE pg_default;

create index IF not exists idx_enterprise_transactions_org_id on public.enterprise_transactions using btree (org_id) TABLESPACE pg_default;

create index IF not exists idx_enterprise_transactions_status on public.enterprise_transactions using btree (status) TABLESPACE pg_default;

create index IF not exists idx_enterprise_transactions_created_at on public.enterprise_transactions using btree (created_at desc) TABLESPACE pg_default;