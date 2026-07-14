use tauri::{
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Manager,
};
use tauri_plugin_notification::NotificationExt;
use std::process::{Command, Child};
use std::sync::Mutex;

struct AppState {
    host_process: Mutex<Option<Child>>,
}

#[tauri::command]
async fn connect_to_signal(url: String) -> Result<String, String> {
    // Health check the signaling server
    let http_url = url
        .replace("ws://", "http://")
        .replace("wss://", "https://");
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(5))
        .build()
        .map_err(|e| e.to_string())?;
    let resp = client
        .get(format!("{}/health", http_url.trim_end_matches("/ws")))
        .send()
        .await
        .map_err(|e| format!("Connection failed: {}", e))?;
    let status = resp.text().await.map_err(|e| e.to_string())?;
    Ok(status)
}

#[tauri::command]
async fn start_host(signal_url: String, name: String, password: String) -> Result<String, String> {
    let mut cmd = Command::new("remotty");
    cmd.args(["host", "--signal", &signal_url, "--name", &name]);
    if !password.is_empty() {
        cmd.args(["--master-password", &password]);
    }
    let child = cmd.spawn().map_err(|e| format!("Failed to start host: {}", e))?;
    Ok(format!("Host started with PID: {}", child.id()))
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_notification::init())
        .manage(AppState {
            host_process: Mutex::new(None),
        })
        .setup(|app| {
            // Build system tray
            let quit = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;
            let show = MenuItem::with_id(app, "show", "Show remotty", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&show, &quit])?;

            TrayIconBuilder::new()
                .icon(app.default_window_icon().unwrap().clone())
                .menu(&menu)
                .tooltip("remotty")
                .on_menu_event(|app, event| {
                    match event.id.as_ref() {
                        "quit" => {
                            app.exit(0);
                        }
                        "show" => {
                            if let Some(window) = app.get_webview_window("main") {
                                let _ = window.show();
                                let _ = window.set_focus();
                            }
                        }
                        _ => {}
                    }
                })
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click {
                        button: MouseButton::Left,
                        button_state: MouseButtonState::Up,
                        ..
                    } = event {
                        if let Some(window) = tray.app_handle().get_webview_window("main") {
                            let _ = window.show();
                            let _ = window.set_focus();
                        }
                    }
                })
                .build(app)?;

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![connect_to_signal, start_host])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
