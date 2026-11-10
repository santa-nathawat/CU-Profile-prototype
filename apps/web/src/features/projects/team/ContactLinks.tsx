'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Alert,
  Button,
  Field,
  FormAlert,
  Input,
  Section,
} from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import type { ProjectDetail } from '@/lib/types';
import { useTeamAction } from './use-team-action';

export function ContactLinks({ project }: { project: ProjectDetail }) {
  if (!project.viewer.isMember || !project.contact) return null;
  const { lineUrl, discordUrl } = project.contact;
  return (
    <Section title="Team chat" description="Private links for current members.">
      {lineUrl || discordUrl ? (
        <div className="flex flex-wrap gap-4">
          {lineUrl ? (
            <a
              href={lineUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium text-accent-ink underline underline-offset-2"
            >
              Open LINE
            </a>
          ) : null}
          {discordUrl ? (
            <a
              href={discordUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium text-accent-ink underline underline-offset-2"
            >
              Open Discord
            </a>
          ) : null}
        </div>
      ) : (
        <p className="text-sm text-ink-muted">
          The lead has not added a team link yet.
        </p>
      )}
    </Section>
  );
}

export function ContactLinksEditor({ project }: { project: ProjectDetail }) {
  const router = useRouter();
  const form = useTeamAction();
  const [lineUrl, setLineUrl] = useState(project.contact?.lineUrl ?? '');
  const [discordUrl, setDiscordUrl] = useState(
    project.contact?.discordUrl ?? '',
  );
  const [saved, setSaved] = useState(false);
  if (!project.viewer.isLead) return null;
  async function save(event: React.FormEvent) {
    event.preventDefault();
    setSaved(false);
    const result = await form.run(() =>
      clientApi.put(`/projects/${project.id}/contact`, {
        lineUrl: lineUrl.trim() || null,
        discordUrl: discordUrl.trim() || null,
      }),
    );
    if (result.ok) {
      setSaved(true);
      router.refresh();
    }
  }
  return (
    <div className="space-y-4">
      <ContactLinks project={project} />
      <Section
        title="Edit team links"
        description="Only current members can see these links. Leave either field empty to clear it."
      >
        <form onSubmit={save} noValidate className="space-y-4">
          <Field
            label="LINE invite link"
            hint="HTTPS on line.me or a line.me subdomain"
            error={form.fields.lineUrl}
          >
            <Input
              type="url"
              value={lineUrl}
              onChange={(event) => {
                setSaved(false);
                setLineUrl(event.target.value);
              }}
              placeholder="https://line.me/ti/g/your-group"
              disabled={form.busy}
            />
          </Field>
          <Field
            label="Discord invite link"
            hint="HTTPS on discord.gg or discord.com/invite"
            error={form.fields.discordUrl}
          >
            <Input
              type="url"
              value={discordUrl}
              onChange={(event) => {
                setSaved(false);
                setDiscordUrl(event.target.value);
              }}
              placeholder="https://discord.gg/your-invite"
              disabled={form.busy}
            />
          </Field>
          <FormAlert form={form} />
          {saved ? <Alert tone="success">Team links saved.</Alert> : null}
          <Button type="submit" variant="secondary" loading={form.busy}>
            Save team links
          </Button>
        </form>
      </Section>
    </div>
  );
}
