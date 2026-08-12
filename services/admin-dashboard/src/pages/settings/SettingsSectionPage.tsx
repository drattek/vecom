interface SettingsSectionPageProps {
  title: string
  description: string
}

export function SettingsSectionPage({ title, description }: SettingsSectionPageProps) {
  return (
    <section className="rounded-xl border bg-card p-6">
      <h2 className="text-xl font-semibold text-foreground">{title}</h2>
      <p className="mt-2 text-sm text-muted-foreground">{description}</p>
    </section>
  )
}
