const adminTokenInput = document.getElementById("adminToken");
const saveBtn = document.getElementById("saveBtn");
const clearBtn = document.getElementById("clearBtn");
const statusEl = document.getElementById("status");

async function loadCurrentToken() {
  const data = await chrome.storage.local.get(["adminToken"]);
  if (data.adminToken) {
    adminTokenInput.value = data.adminToken;
  }
}

function showStatus(message, isError) {
  statusEl.textContent = message;
  statusEl.className = isError ? "status error" : "status success";
  setTimeout(() => { statusEl.className = "status"; }, 3000);
}

saveBtn.addEventListener("click", async () => {
  const token = adminTokenInput.value.trim();
  if (!token) {
    showStatus("Please enter a token.", true);
    return;
  }
  await chrome.storage.local.set({ adminToken: token });
  showStatus("Admin token saved.", false);
});

clearBtn.addEventListener("click", async () => {
  adminTokenInput.value = "";
  await chrome.storage.local.remove("adminToken");
  showStatus("Admin token cleared.", false);
});

loadCurrentToken();
