#!/usr/bin/env python3
"""
Xcode project generator for remotty macOS app.
Generates a proper .xcodeproj from Swift source files.
Usage: python3 scripts/gen-xcode-project.py
"""

import os
import plistlib
import uuid
import shutil

PROJECT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MACOS_DIR = os.path.join(PROJECT_DIR, "remotty-macOS")
XCODE_PROJ = os.path.join(PROJECT_DIR, "remotty-macOS", "remotty.xcodeproj")
PBX_FILE = os.path.join(XCODE_PROJ, "project.pbxproj")

def new_id():
    return uuid.uuid4().hex.upper()[:24]

def build_pbx():
    """Build the project.pbxproj dictionary."""
    
    # File References
    files = {
        "remottyMenuBarApp.swift": new_id(),
        "MenuBarView.swift": new_id(),
        "SettingsView.swift": new_id(),
        "Info.plist": new_id(),
    }
    
    # Build phases
    sources_id = new_id()
    frameworks_id = new_id()
    resources_id = new_id()
    
    # Build configuration
    debug_config_id = new_id()
    release_config_id = new_id()
    config_list_id = new_id()
    
    # Target
    target_id = new_id()
    product_ref_id = new_id()
    
    # Native target reference
    native_target_id = new_id()
    
    # Group
    main_group_id = new_id()
    src_group_id = new_id()
    
    # Product group
    products_group_id = new_id()
    
    # Root object
    root_id = new_id()
    
    pbx = {
        "archiveVersion": 1,
        "classes": {},
        "objectVersion": 56,
        "objects": {
            # === PBXBuildFile ===
        }
    }
    
    # Add file references and build files
    build_files = {}
    for name, file_id in list(files.items()):
        pbx["objects"][file_id] = {
            "isa": "PBXFileReference",
            "lastKnownFileType": "sourcecode.swift" if name.endswith(".swift") else "text.plist.xml",
            "path": name,
            "sourceTree": "<group>",
        }
        
        build_file_id = new_id()
        build_files[name] = build_file_id
        pbx["objects"][build_file_id] = {
            "isa": "PBXBuildFile",
            "fileRef": file_id,
        }
    
    # === PBXGroup ===
    pbx["objects"][src_group_id] = {
        "isa": "PBXGroup",
        "children": [
            files["remottyMenuBarApp.swift"],
            files["MenuBarView.swift"],
            files["SettingsView.swift"],
            files["Info.plist"],
        ],
        "name": "remotty",
        "sourceTree": "<group>",
    }
    
    pbx["objects"][products_group_id] = {
        "isa": "PBXGroup",
        "children": [product_ref_id],
        "name": "Products",
        "sourceTree": "<group>",
    }
    
    pbx["objects"][main_group_id] = {
        "isa": "PBXGroup",
        "children": [src_group_id, products_group_id],
        "sourceTree": "<group>",
    }
    
    # === PBXProductReference ===
    pbx["objects"][product_ref_id] = {
        "isa": "PBXFileReference",
        "explicitFileType": "wrapper.application",
        "includeInIndex": 0,
        "path": "remotty.app",
        "sourceTree": "BUILT_PRODUCTS_DIR",
    }
    
    # === PBXSourcesBuildPhase ===
    pbx["objects"][sources_id] = {
        "isa": "PBXSourcesBuildPhase",
        "buildActionMask": 2147483647,
        "files": [
            build_files["remottyMenuBarApp.swift"],
            build_files["MenuBarView.swift"],
            build_files["SettingsView.swift"],
        ],
        "runOnlyForDeploymentPostprocessing": 0,
    }
    
    # === PBXFrameworksBuildPhase ===
    pbx["objects"][frameworks_id] = {
        "isa": "PBXFrameworksBuildPhase",
        "buildActionMask": 2147483647,
        "files": [],
        "runOnlyForDeploymentPostprocessing": 0,
    }
    
    # === PBXResourcesBuildPhase ===
    pbx["objects"][resources_id] = {
        "isa": "PBXResourcesBuildPhase",
        "buildActionMask": 2147483647,
        "files": [build_files["Info.plist"]],
        "runOnlyForDeploymentPostprocessing": 0,
    }
    
    # === XCBuildConfiguration (Debug) ===
    pbx["objects"][debug_config_id] = {
        "isa": "XCBuildConfiguration",
        "buildSettings": {
            "ASSETCATALOG_COMPILER_APPICON_NAME": "",
            "CODE_SIGN_STYLE": "Manual",
            "CODE_SIGN_IDENTITY": "-",  # Ad-hoc signing
            "COMBINE_HIDPI_IMAGES": "YES",
            "CURRENT_PROJECT_VERSION": 1,
            "GENERATE_INFOPLIST_FILE": "YES",
            "INFOPLIST_FILE": "Info.plist",
            "INFOPLIST_KEY_LSUIElement": "YES",
            "INFOPLIST_KEY_NSHighResolutionCapable": "YES",
            "LD_RUNPATH_SEARCH_PATHS": [
                "$(inherited)",
                "@executable_path/../Frameworks",
            ],
            "MACOSX_DEPLOYMENT_TARGET": "14.0",
            "MARKETING_VERSION": "0.5.0",
            "PRODUCT_BUNDLE_IDENTIFIER": "com.remotty.macos",
            "PRODUCT_NAME": "$(TARGET_NAME)",
            "SDKROOT": "macosx",
            "SWIFT_VERSION": "5.0",
            "SWIFT_OPTIMIZATION_LEVEL": "-Onone",
        },
        "name": "Debug",
    }
    
    # === XCBuildConfiguration (Release) ===
    pbx["objects"][release_config_id] = {
        "isa": "XCBuildConfiguration",
        "buildSettings": {
            "ASSETCATALOG_COMPILER_APPICON_NAME": "",
            "CODE_SIGN_STYLE": "Manual",
            "CODE_SIGN_IDENTITY": "-",  # Ad-hoc signing
            "COMBINE_HIDPI_IMAGES": "YES",
            "CURRENT_PROJECT_VERSION": 1,
            "GENERATE_INFOPLIST_FILE": "YES",
            "INFOPLIST_FILE": "Info.plist",
            "INFOPLIST_KEY_LSUIElement": "YES",
            "INFOPLIST_KEY_NSHighResolutionCapable": "YES",
            "LD_RUNPATH_SEARCH_PATHS": [
                "$(inherited)",
                "@executable_path/../Frameworks",
            ],
            "MACOSX_DEPLOYMENT_TARGET": "14.0",
            "MARKETING_VERSION": "0.5.0",
            "PRODUCT_BUNDLE_IDENTIFIER": "com.remotty.macos",
            "PRODUCT_NAME": "$(TARGET_NAME)",
            "SDKROOT": "macosx",
            "SWIFT_VERSION": "5.0",
            "SWIFT_OPTIMIZATION_LEVEL": "-O",
        },
        "name": "Release",
    }
    
    # === XCConfigurationList ===
    pbx["objects"][config_list_id] = {
        "isa": "XCConfigurationList",
        "buildConfigurations": [debug_config_id, release_config_id],
        "defaultConfigurationIsVisible": 0,
        "defaultConfigurationName": "Release",
    }
    
    # === PBXNativeTarget ===
    pbx["objects"][native_target_id] = {
        "isa": "PBXNativeTarget",
        "buildConfigurationList": config_list_id,
        "buildPhases": [sources_id, frameworks_id, resources_id],
        "buildRules": [],
        "dependencies": [],
        "name": "remotty",
        "productName": "remotty",
        "productReference": product_ref_id,
        "productType": "com.apple.product-type.application",
    }
    
    # === PBXProject ===
    pbx["objects"][root_id] = {
        "isa": "PBXProject",
        "attributes": {
            "BuildIndependentTargetsInParallel": 1,
            "LastSwiftUpdateCheck": 1530,
            "LastUpgradeCheck": 1530,
        },
        "buildConfigurationList": new_id(),  # Project-level config
        "compatibilityVersion": "Xcode 14.0",
        "developmentRegion": "en",
        "hasScannedForEncodings": 0,
        "knownRegions": ["en", "Base"],
        "mainGroup": main_group_id,
        "productRefGroup": products_group_id,
        "projectDirPath": "",
        "projectRoot": "",
        "targets": [native_target_id],
    }
    
    # Project-level build configs
    proj_debug_id = new_id()
    proj_release_id = new_id()
    proj_config_list_id = new_id()
    
    pbx["objects"][proj_debug_id] = {
        "isa": "XCBuildConfiguration",
        "buildSettings": {
            "ALWAYS_SEARCH_USER_PATHS": "NO",
            "CLANG_ANALYZER_NONNULL": "YES",
            "CLANG_CXX_LANGUAGE_STANDARD": "gnu++20",
            "CLANG_ENABLE_MODULES": "YES",
            "CLANG_ENABLE_OBJC_ARC": "YES",
            "COPY_PHASE_STRIP": "NO",
            "DEBUG_INFORMATION_FORMAT": "dwarf",
            "ENABLE_STRICT_OBJC_MSGSEND": "YES",
            "GCC_DYNAMIC_NO_PIC": "NO",
            "GCC_OPTIMIZATION_LEVEL": "0",
            "GCC_PREPROCESSOR_DEFINITIONS": ["DEBUG=1", "$(inherited)"],
            "MACOSX_DEPLOYMENT_TARGET": "14.0",
            "MTL_ENABLE_DEBUG_INFO": "INCLUDE_SOURCE",
            "ONLY_ACTIVE_ARCH": "YES",
            "SDKROOT": "macosx",
            "SWIFT_ACTIVE_COMPILATION_CONDITIONS": "DEBUG",
            "SWIFT_OPTIMIZATION_LEVEL": "-Onone",
        },
        "name": "Debug",
    }
    
    pbx["objects"][proj_release_id] = {
        "isa": "XCBuildConfiguration",
        "buildSettings": {
            "ALWAYS_SEARCH_USER_PATHS": "NO",
            "CLANG_ANALYZER_NONNULL": "YES",
            "CLANG_CXX_LANGUAGE_STANDARD": "gnu++20",
            "CLANG_ENABLE_MODULES": "YES",
            "CLANG_ENABLE_OBJC_ARC": "YES",
            "COPY_PHASE_STRIP": "NO",
            "DEBUG_INFORMATION_FORMAT": "dwarf-with-dsym",
            "ENABLE_NS_ASSERTIONS": "NO",
            "ENABLE_STRICT_OBJC_MSGSEND": "YES",
            "GCC_OPTIMIZATION_LEVEL": "s",
            "MACOSX_DEPLOYMENT_TARGET": "14.0",
            "MTL_ENABLE_DEBUG_INFO": "NO",
            "SDKROOT": "macosx",
            "SWIFT_COMPILATION_MODE": "wholemodule",
            "SWIFT_OPTIMIZATION_LEVEL": "-O",
        },
        "name": "Release",
    }
    
    pbx["objects"][proj_config_list_id] = {
        "isa": "XCConfigurationList",
        "buildConfigurations": [proj_debug_id, proj_release_id],
        "defaultConfigurationIsVisible": 0,
        "defaultConfigurationName": "Release",
    }
    
    # Fix project config list reference
    pbx["objects"][root_id]["buildConfigurationList"] = proj_config_list_id
    
    return pbx


def main():
    # Create Xcode project directory
    os.makedirs(XCODE_PROJ, exist_ok=True)
    
    pbx = build_pbx()
    
    # Write project.pbxproj
    with open(PBX_FILE, "wb") as f:
        f.write(b"// !$*UTF8*$!\n")
        plistlib.dump(pbx, f, sort_keys=False)
    
    print(f"✅ Xcode project generated: {XCODE_PROJ}")
    print(f"   File: {PBX_FILE}")
    print(f"")
    print(f"To open in Xcode:")
    print(f"   open {XCODE_PROJ}")
    print(f"")
    print(f"To build:")
    print(f"   xcodebuild -project {XCODE_PROJ} -scheme remotty -configuration Release build")


if __name__ == "__main__":
    main()
