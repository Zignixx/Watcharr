<script lang="ts">
	import axios from "axios";
	import type { PublicUser } from "@/types";
	import { baseURL } from "@/lib/util/api";
	import Spinner from "@/lib/Spinner.svelte";
	import Icon from "@/lib/Icon.svelte";
	import { onMount } from "svelte";

	let users: PublicUser[] = $state([]);
	let loading = $state(true);
	let error = $state("");

	onMount(async () => {
		try {
			const resp = await axios.get("/user/list");
			users = resp.data || [];
		} catch (err: any) {
			console.error("Failed to load users:", err);
			error = "Failed to load users";
		}
		loading = false;
	});
</script>

<svelte:head>
	<title>Users</title>
</svelte:head>

<div class="users-page">
	<h2>Users</h2>

	{#if loading}
		<Spinner />
	{:else if error}
		<div class="error-msg">{error}</div>
	{:else if users.length === 0}
		<div class="empty">
			<Icon i="person" wh={60} />
			<p>No public users found</p>
		</div>
	{:else}
		<div class="users-grid">
			{#each users as user (user.id)}
				<a href="/lists/{user.id}/{user.username}" class="user-card">
					<div class="user-avatar">
						{#if user.avatar?.path}
							<img src={`${baseURL}/${user.avatar.path}`} alt={user.username} />
						{:else}
							<Icon i="person" wh={40} />
						{/if}
					</div>
					<div class="user-info">
						<span class="user-name">{user.username}</span>
						{#if user.bio}
							<span class="user-bio">{user.bio}</span>
						{/if}
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>

<style lang="scss">
	.users-page {
		padding: 24px;
		max-width: 900px;
		margin: 0 auto;

		h2 {
			margin: 0 0 20px;
			font-size: 24px;
			font-weight: 700;
		}
	}

	.error-msg {
		color: $error;
		text-align: center;
		padding: 20px;
	}

	.empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		padding: 40px 0;
		opacity: 0.5;

		p {
			margin: 0;
			font-size: 14px;
		}
	}

	.users-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
		gap: 12px;
	}

	.user-card {
		display: flex;
		align-items: center;
		gap: 14px;
		padding: 14px 16px;
		border-radius: 12px;
		border: 1px solid rgba(128, 128, 128, 0.15);
		background: rgba(128, 128, 128, 0.04);
		text-decoration: none;
		color: $text-color;
		transition: all 180ms ease;

		&:hover {
			border-color: rgba(128, 128, 128, 0.3);
			background: rgba(128, 128, 128, 0.08);
			transform: translateY(-1px);
			box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
		}
	}

	.user-avatar {
		width: 48px;
		height: 48px;
		border-radius: 50%;
		overflow: hidden;
		flex-shrink: 0;
		background: rgba(128, 128, 128, 0.1);
		display: flex;
		align-items: center;
		justify-content: center;

		img {
			width: 100%;
			height: 100%;
			object-fit: cover;
		}
	}

	.user-info {
		display: flex;
		flex-direction: column;
		gap: 2px;
		overflow: hidden;
	}

	.user-name {
		font-size: 15px;
		font-weight: 600;
	}

	.user-bio {
		font-size: 12px;
		opacity: 0.6;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	@media screen and (max-width: 600px) {
		.users-page {
			padding: 16px 12px;
		}

		.users-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
