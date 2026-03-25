<script lang="ts">
	import Checkbox from "@/lib/Checkbox.svelte";
	import DropDown from "@/lib/DropDown.svelte";
	import Modal from "@/lib/Modal.svelte";
	import Setting from "@/lib/settings/Setting.svelte";
	import SettingsList from "@/lib/settings/SettingsList.svelte";
	import { userHasPermission } from "@/lib/util/helpers";
	import { notify } from "@/lib/util/notify";
	import { UserPermission, UserType, type ManagedUser } from "@/types";
	import axios from "axios";

	interface UpdateUserRequest {
		permissions?: number;
		type?: UserType;
		username?: string;
	}

	interface Props {
		user: ManagedUser;
		onClose: () => void;
	}

	let { user = $bindable(), onClose }: Props = $props();

	let error: string | undefined = $state();
	let formDisabled = false;

	// Things we have changed
	let changedPerms = false;
	let originalUser = structuredClone(user);
	let newUsername = $state(user.username);
	let confirmDelete = $state(false);
	let deleteLoading = $state(false);

	async function save() {
		const changedType = user.type !== originalUser.type;
		const changedUsername = newUsername !== originalUser.username;
		// If nothing changed.. error
		if (!changedPerms && !changedType && !changedUsername) {
			error = "Nothing has been changed";
			return;
		}
		error = undefined;
		try {
			const toUpdate: UpdateUserRequest = {};
			if (changedPerms) {
				toUpdate["permissions"] = user.permissions;
			}
			if (changedType) {
				toUpdate["type"] = user.type;
			}
			if (changedUsername) {
				if (!newUsername || newUsername.trim().length === 0) {
					error = "Username cannot be empty";
					return;
				}
				toUpdate["username"] = newUsername.trim();
			}
			const res = await axios.post(`/server/users/${user.id}`, toUpdate);
			if (res.status === 200) {
				notify({
					type: "success",
					text: "Changes saved!",
				});
				onClose();
			}
		} catch (err: any) {
			console.error("Failed to save user!", err);
			error = `Failed to save`;
			if (err?.response?.data?.error) {
				error = err.response.data.error;
			}
		}
	}

	async function deleteUser() {
		deleteLoading = true;
		error = undefined;
		try {
			await axios.delete(`/server/users/${user.id}`);
			notify({
				type: "success",
				text: `User "${user.username}" deleted!`,
			});
			onClose();
		} catch (err: any) {
			console.error("Failed to delete user!", err);
			error = "Failed to delete user";
			if (err?.response?.data?.error) {
				error = err.response.data.error;
			}
		}
		deleteLoading = false;
		confirmDelete = false;
	}

	function userTogglePermission(perm: UserPermission) {
		user.permissions ^= perm;
		changedPerms = true;
	}
</script>

<Modal
	title={`Edit User`}
	desc={`Configuring ${originalUser.username}`}
	maxWidth="500px"
	{onClose}
>
	{#if error}
		<span class="error">{error}!</span>
	{/if}

	<SettingsList>
		<h3 class="norm">Username</h3>

		<Setting
			title="Rename User"
			desc="Change this user's username."
			row
		>
			<input
				type="text"
				bind:value={newUsername}
				placeholder="New username"
				maxlength="50"
				class="username-input"
			/>
		</Setting>

		<h3 class="norm">Permissions</h3>

		<Setting
			title="Admin"
			desc="Give user admin, overrides all other permissions."
			row
		>
			<Checkbox
				name="USER_PERM_ADMIN"
				value={userHasPermission(user.permissions, UserPermission.PERM_ADMIN)}
				toggled={() => {
					userTogglePermission(UserPermission.PERM_ADMIN);
				}}
			/>
		</Setting>

		<Setting
			title="Request Content"
			desc="Give user permission to request content."
			row
		>
			<Checkbox
				name="USER_PERM_REQUEST_CONTENT"
				value={userHasPermission(
					user.permissions,
					UserPermission.PERM_REQUEST_CONTENT,
				)}
				toggled={() => {
					userTogglePermission(UserPermission.PERM_REQUEST_CONTENT);
				}}
			/>
		</Setting>

		<Setting
			title="Auto Approve Content Request"
			desc="Auto approve user's content requests."
			row
		>
			<Checkbox
				name="PERM_REQUEST_CONTENT_AUTO_APPROVE"
				value={userHasPermission(
					user.permissions,
					UserPermission.PERM_REQUEST_CONTENT_AUTO_APPROVE,
				)}
				toggled={() => {
					userTogglePermission(
						UserPermission.PERM_REQUEST_CONTENT_AUTO_APPROVE,
					);
				}}
			/>
		</Setting>

		<h3 class="norm">Other</h3>

		<Setting
			title="User Type"
			desc="The type of this user, affects how they login and certain features. Currently only possible to swap between Watcharr/Proxy types."
		>
			{#if !user.type || user.type === UserType.Proxy}
				<DropDown
					placeholder="Unknown"
					bind:active={user.type}
					options={[
						{
							id: 0,
							value: "Watcharr",
						},
						// {
						//   id: UserType.Jellyfin,
						//   value: "Jellyfin",
						// },
						// {
						//   id: UserType.Plex,
						//   value: "Plex",
						// },
						{
							id: UserType.Proxy,
							value: "Proxy",
						},
					]}
					isDropDownItem={true}
				/>
			{:else}
				<p>This option is not supported for this user type yet.</p>
			{/if}
		</Setting>

		<h3 class="norm">Danger Zone</h3>

		<Setting
			title="Delete User"
			desc="Permanently delete this user and all their data. This cannot be undone."
			row
		>
			{#if !confirmDelete}
				<button class="delete-btn" onclick={() => (confirmDelete = true)}>Delete</button>
			{:else}
				<div class="confirm-delete">
					<span>Are you sure?</span>
					<button class="delete-btn" onclick={deleteUser} disabled={deleteLoading}>
						{deleteLoading ? "Deleting..." : "Yes, Delete"}
					</button>
					<button onclick={() => (confirmDelete = false)}>Cancel</button>
				</div>
			{/if}
		</Setting>

		<div class="btns">
			<button onclick={() => save()}>Save</button>
		</div>
	</SettingsList>
</Modal>

<style lang="scss">
	.btns {
		display: flex;
		flex-flow: row;
		gap: 10px;

		:first-child {
			margin-left: auto;
		}

		button {
			width: max-content;
			padding-left: 15px;
			padding-right: 15px;
		}
	}

	.error {
		position: sticky;
		top: 0;
		display: flex;
		justify-content: center;
		width: 100%;
		padding: 10px;
		background-color: rgb(221, 48, 48);
		text-transform: capitalize;
		color: white;
		margin-bottom: 15px;
	}

	.username-input {
		padding: 8px 12px;
		border-radius: 5px;
		border: 1px solid rgba(255, 255, 255, 0.15);
		background: rgba(255, 255, 255, 0.05);
		color: inherit;
		font-size: 14px;
		width: 200px;
	}

	.delete-btn {
		background-color: rgb(221, 48, 48) !important;
		color: white;

		&:hover {
			background-color: rgb(190, 30, 30) !important;
		}
	}

	.confirm-delete {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;

		span {
			font-weight: bold;
			color: rgb(221, 48, 48);
		}

		button {
			padding: 6px 12px;
			font-size: 13px;
		}
	}
</style>
