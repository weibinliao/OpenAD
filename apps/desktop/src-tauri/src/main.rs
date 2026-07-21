use tauri::{CustomMenuItem, SystemTray, SystemTrayMenu, SystemTrayEvent, Manager, AppHandle};
use tauri::api::dialog;

#[tauri::command]
async fn select_folder() -> Result<String, String> {
    match dialog::blocking::FileDialogBuilder::new()
        .set_title("Select Folder to Scan")
        .pick_folder() {
        Some(path) => Ok(path.to_string_lossy().to_string()),
        None => Err("No folder selected".to_string()),
    }
}

#[tauri::command]
async fn scan_permissions(path: String) -> Result<serde_json::Value, String> {
    // Call the Go backend API
    let client = reqwest::Client::new();
    let response = client
        .post("http://localhost:18080/api/scan")
        .json(&serde_json::json!({"path": path}))
        .send()
        .await
        .map_err(|e| e.to_string())?;

    let result = response.json::<serde_json::Value>()
        .await
        .map_err(|e| e.to_string())?;

    Ok(result)
}

fn create_system_tray() -> SystemTray {
    let quit = CustomMenuItem::new("quit".to_string(), "Quit");
    let show = CustomMenuItem::new("show".to_string(), "Show");
    let tray_menu = SystemTrayMenu::new()
        .add_item(show)
        .add_native_item(tauri::SystemTrayMenuItem::Separator)
        .add_item(quit);

    SystemTray::new().with_menu(tray_menu)
}

fn handle_system_tray_event(app: &AppHandle, event: SystemTrayEvent) {
    match event {
        SystemTrayEvent::LeftClick { .. } => {
            let window = app.get_window("main").unwrap();
            window.show().unwrap();
            window.set_focus().unwrap();
        }
        SystemTrayEvent::MenuItemClick { id, .. } => {
            match id.as_str() {
                "quit" => {
                    std::process::exit(0);
                }
                "show" => {
                    let window = app.get_window("main").unwrap();
                    window.show().unwrap();
                    window.set_focus().unwrap();
                }
                _ => {}
            }
        }
        _ => {}
    }
}

fn main() {
    tauri::Builder::default()
        .system_tray(create_system_tray())
        .on_system_tray_event(handle_system_tray_event)
        .invoke_handler(tauri::generate_handler![select_folder, scan_permissions])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
