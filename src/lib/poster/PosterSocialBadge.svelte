<script lang="ts">
	import type { WatchedStatus } from "@/types";
	import Icon from "../Icon.svelte";
	import { watchedStatuses } from "../util/helpers";

	interface SocialUser {
		followedUser: { id: number; username: string };
		status: WatchedStatus;
	}

	interface Props {
		socialData: SocialUser[];
	}

	let { socialData }: Props = $props();

	let planned = $derived(socialData.filter((s) => s.status === "PLANNED"));
	let finished = $derived(socialData.filter((s) => s.status === "FINISHED"));
	let dropped = $derived(socialData.filter((s) => s.status === "DROPPED"));
	let watching = $derived(socialData.filter((s) => s.status === "WATCHING"));

	let expanded = $state(false);
</script>

{#if socialData.length > 0}
	<div class="social-badge-wrap">
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="social-badge" onclick={(e) => { e.stopPropagation(); e.preventDefault(); expanded = !expanded; }}>
			<Icon i="people" wh={12} />
			<span>{socialData.length}</span>
		</div>

		{#if expanded}
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="social-popup" onclick={(e) => { e.stopPropagation(); e.preventDefault(); }}>
				{#if finished.length > 0}
					<div class="social-group">
						<div class="social-group-header">
							<Icon i={watchedStatuses["FINISHED"]} wh={14} />
							<span>Finished ({finished.length})</span>
						</div>
						{#each finished as f}
							<span class="social-user">{f.followedUser.username}</span>
						{/each}
					</div>
				{/if}
				{#if watching.length > 0}
					<div class="social-group">
						<div class="social-group-header">
							<Icon i={watchedStatuses["WATCHING"]} wh={14} />
							<span>Watching ({watching.length})</span>
						</div>
						{#each watching as w}
							<span class="social-user">{w.followedUser.username}</span>
						{/each}
					</div>
				{/if}
				{#if planned.length > 0}
					<div class="social-group">
						<div class="social-group-header">
							<Icon i={watchedStatuses["PLANNED"]} wh={14} />
							<span>Planned ({planned.length})</span>
						</div>
						{#each planned as p}
							<span class="social-user">{p.followedUser.username}</span>
						{/each}
					</div>
				{/if}
				{#if dropped.length > 0}
					<div class="social-group">
						<div class="social-group-header">
							<Icon i={watchedStatuses["DROPPED"]} wh={14} />
							<span>Dropped ({dropped.length})</span>
						</div>
						{#each dropped as d}
							<span class="social-user">{d.followedUser.username}</span>
						{/each}
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/if}

<style lang="scss">
	.social-badge-wrap {
		position: absolute;
		top: 6px;
		left: 6px;
		z-index: 3;
	}

	.social-badge {
		display: flex;
		align-items: center;
		gap: 4px;
		background: rgba(0, 0, 0, 0.75);
		backdrop-filter: blur(4px);
		border-radius: 6px;
		padding: 3px 6px;
		cursor: pointer;
		fill: $accent-color;
		color: white;
		font-size: 11px;
		font-weight: 600;
		transition: background 150ms ease;

		&:hover {
			background: rgba(0, 0, 0, 0.9);
		}
	}

	.social-popup {
		position: absolute;
		top: 100%;
		left: 0;
		margin-top: 4px;
		background: $bg-color;
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 10px;
		padding: 10px;
		min-width: 160px;
		max-width: 220px;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
		display: flex;
		flex-direction: column;
		gap: 8px;
		z-index: 10;
	}

	.social-group {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.social-group-header {
		display: flex;
		align-items: center;
		gap: 5px;
		fill: rgba(255, 255, 255, 0.6);
		color: rgba(255, 255, 255, 0.6);
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		margin-bottom: 2px;
	}

	.social-user {
		font-size: 12px;
		color: rgba(255, 255, 255, 0.85);
		padding: 2px 0 2px 19px;
	}
</style>
